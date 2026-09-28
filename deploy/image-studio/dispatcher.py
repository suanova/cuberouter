# Copyright (C) 2023-2026 QuantumNous
# Copyright (C) 2026 CubeRouter
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as
# published by the Free Software Foundation, either version 3 of the
# License, or (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program. If not, see <https://www.gnu.org/licenses/>.
#
# For commercial licensing, please contact support@quantumnous.com

"""Authenticated, bounded RAM queue. No prompts, images or request bodies are logged."""
import base64
import binascii
import hmac
import json
import math
import os
import re
import secrets
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError
from urllib.request import Request, ProxyHandler, build_opener

from queue_core import Dispatcher, Rejected, BusyBeforeAccept, UncertainExecution

HTTP = build_opener(ProxyHandler({}))


def validate(p):
    if not isinstance(p, dict) or set(p) - {'model', 'mode', 'prompt', 'user_prompt', 'steps', 'seed', 'cfg', 'width', 'height', 'images'}:
        raise Rejected(400, 'invalid_parameters')
    if p.get('model') != '2.1' or p.get('mode') not in ('create', 'edit'):
        raise Rejected(400, 'invalid_model_or_mode')
    if not isinstance(p.get('prompt'), str) or not 1 <= len(p['prompt'].strip()) <= 16000:
        raise Rejected(400, 'invalid_prompt')
    if not isinstance(p.get('user_prompt', ''), str) or len(p.get('user_prompt', '')) > 16000:
        raise Rejected(400, 'invalid_user_prompt')
    for key, low, high in [('steps', 1, 60), ('seed', 0, 2**53-1), ('width', 256, 1664), ('height', 256, 1664)]:
        if type(p.get(key)) is not int or not low <= p[key] <= high:
            raise Rejected(400, 'invalid_' + key)
    if p['width'] % 32 or p['height'] % 32 or p['width'] * p['height'] > 2097152:
        raise Rejected(400, 'invalid_dimensions')
    if type(p.get('cfg')) not in (int, float) or not math.isfinite(p['cfg']) or not 0 <= p['cfg'] <= 10:
        raise Rejected(400, 'invalid_cfg')
    images = p.get('images', [])
    if not isinstance(images, list) or len(images) > 10 or (p['mode'] == 'edit' and not images) or (p['mode'] == 'create' and images):
        raise Rejected(400, 'invalid_images')
    for value in images:
        try:
            if not isinstance(value, str) or len(value) > 14*1024*1024 or len(base64.b64decode(value, validate=True)) > 10*1024*1024:
                raise ValueError()
        except (ValueError, binascii.Error):
            raise Rejected(400, 'invalid_image') from None
    return p


def ready(channel):
    with HTTP.open(channel['url'] + '/health', timeout=3) as response:
        state = json.load(response)
    return state.get('state') == 'ready' and state.get('model') == '2.1'


def run(channel, payload):
    try:
        parameters = json.loads(payload)
        parameters.pop('user_prompt', None)
        payload = json.dumps(parameters, allow_nan=False).encode()
        request = Request(channel['url'] + '/generate', data=payload, headers={'Content-Type': 'application/json'})
        with HTTP.open(request, timeout=1500) as response:
            data = response.read(24*1024*1024+1)
        if len(data) > 24*1024*1024:
            raise ValueError('worker_result_limit')
        result = json.loads(data)
        image = base64.b64decode(result.pop('image'), validate=True)
        return image, result
    except HTTPError as exc:
        if exc.code in (429, 503):
            raise BusyBeforeAccept() from None
        raise RuntimeError('worker_rejected') from None
    except (OSError, TimeoutError):
        # Never resubmit a request that the worker may already have accepted.
        raise UncertainExecution() from None


def channel_runner(url, token):
    """Queue capacity is separate from CubeRouter's provider channel selection."""
    if len(token) < 32 or not url.startswith(('http://', 'https://')):
        raise ValueError('Configure private relay URL and token')

    def execute(channel, payload):
        request = Request(url.rstrip('/') + '/internal/image-studio/execute', data=payload,
                          headers={'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token,
                                   'X-Image-Owner': channel['owner'], 'X-Image-Job': channel['job_id']})
        try:
            with HTTP.open(request, timeout=1500) as response:
                data = response.read(24*1024*1024+1)
                provider_channel = response.headers.get('X-Image-Studio-Channel')
            if len(data) > 24*1024*1024:
                raise ValueError('relay_result_limit')
            result = json.loads(data)
            images = result.get('data', [])
            if len(images) != 1 or not images[0].get('b64_json'):
                # No URL fetching: the configured image provider must return
                # inline image bytes, keeping content out of persistent URLs.
                raise ValueError('inline_image_result_required')
            image = base64.b64decode(images[0]['b64_json'], validate=True)
            return image, {'provider_channel': provider_channel}
        except HTTPError:
            # A provider's HTTP error is never a licence to generate twice.
            raise RuntimeError('channel_rejected') from None
        except (OSError, TimeoutError):
            raise UncertainExecution() from None
    return execute


class Server(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, address, dispatch, token):
        if len(token) < 32:
            raise ValueError('IMAGE_STUDIO_DISPATCHER_TOKEN must contain at least 32 characters')
        self.dispatch, self.token, self.epoch = dispatch, token, secrets.token_hex(16)
        self.slots = threading.BoundedSemaphore(24)
        super().__init__(address, Handler)

    def process_request(self, request, address):
        if not self.slots.acquire(False):
            self.shutdown_request(request)
            return
        try:
            super().process_request(request, address)
        except Exception:
            self.slots.release()
            raise

    def process_request_thread(self, request, address):
        try:
            super().process_request_thread(request, address)
        finally:
            self.slots.release()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, status, data, content_type='application/json'):
        data = data if isinstance(data, bytes) else json.dumps(data, allow_nan=False).encode()
        self.send_response(status)
        self.send_header('Content-Type', content_type)
        self.send_header('Content-Length', str(len(data)))
        self.send_header('Cache-Control', 'no-store')
        self.send_header('X-Content-Type-Options', 'nosniff')
        self.end_headers()
        try:
            self.wfile.write(data)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def handle_api(self):
        try:
            self.connection.settimeout(30)
            auth = self.headers.get('Authorization', '')
            if not hmac.compare_digest(auth, 'Bearer ' + self.server.token):
                raise Rejected(401, 'dispatcher_auth_required')
            owner = self.headers.get('X-Image-Owner', '')
            if not re.fullmatch(r'cuberouter-[1-9][0-9]{0,18}', owner):
                raise Rejected(403, 'verified_owner_required')
            queue = self.server.dispatch
            if self.command == 'GET':
                if self.path == '/session':
                    return self.reply(200, {'owner': owner, 'epoch': self.server.epoch, 'generation_enabled': True,
                                            'model': '2.1', 'result_ttl': queue.result_ttl, 'storage': 'browser'})
                if self.path == '/pools':
                    return self.reply(200, queue.metrics())
                lookup = re.fullmatch(r'/requests/([a-zA-Z0-9_-]{16,80})', self.path)
                if lookup:
                    return self.reply(200, queue.lookup(owner, lookup[1]))
                match = re.fullmatch(r'/jobs/([0-9a-f]{32})(/result)?', self.path)
                if not match:
                    raise Rejected(404, 'unknown_route')
                if match[2]:
                    return self.reply(200, queue.result(owner, match[1]), 'image/png')
                return self.reply(200, queue.status(owner, match[1]))
            if self.headers.get('Content-Type', '').split(';')[0] != 'application/json':
                raise Rejected(415, 'json_required')
            if self.headers.get('Transfer-Encoding'):
                raise Rejected(400, 'content_length_required')
            length = int(self.headers.get('Content-Length', '0'))
            if not 0 < length <= 16*1024*1024:
                raise Rejected(413, 'body_limit')
            body = self.rfile.read(length)
            if len(body) != length:
                raise Rejected(400, 'incomplete_body')
            parameters = json.loads(body)
            if self.path == '/jobs':
                if self.headers.get('X-Trial-Epoch') != self.server.epoch:
                    raise Rejected(409, 'server_restarted_review_before_resubmit')
                key = self.headers.get('Idempotency-Key', '')
                if not re.fullmatch(r'[a-zA-Z0-9_-]{16,80}', key):
                    raise Rejected(400, 'idempotency_key_required')
                job, created = queue.submit(owner, key, validate(parameters))
                return self.reply(202 if created else 200, job)
            match = re.fullmatch(r'/jobs/([0-9a-f]{32})/(ack|cancel)', self.path)
            if not match:
                raise Rejected(404, 'unknown_route')
            return self.reply(200, getattr(queue, match[2])(owner, match[1]))
        except Rejected as exc:
            self.reply(exc.status, {'error': exc.code})
        except (ValueError, TypeError, TimeoutError):
            self.reply(400, {'error': 'invalid_request'})

    do_GET = do_POST = handle_api


if __name__ == '__main__':
    relay_url = os.environ.get('IMAGE_STUDIO_RELAY_URL', '')
    if relay_url:
        capacity = int(os.environ.get('IMAGE_STUDIO_RELAY_CONCURRENCY', '1'))
        if not 1 <= capacity <= 8:
            raise SystemExit('Relay concurrency must be between 1 and 8')
        channels = [{'id': f'channel-slot-{i+1}', 'model': '2.1'} for i in range(capacity)]
        runner = channel_runner(relay_url, os.environ.get('IMAGE_STUDIO_RELAY_TOKEN', ''))
        health = lambda channel: True
    else:
        channels = json.loads(os.environ['IMAGE_STUDIO_CHANNELS'])
        runner, health = run, ready
    if not channels or any(c.get('model') != '2.1' for c in channels):
        raise SystemExit('Configure Qwen Image 2.1 channels')
    dispatch = Dispatcher(channels, runner, health)
    server = Server((os.environ.get('IMAGE_STUDIO_BIND', '127.0.0.1'),
                     int(os.environ.get('IMAGE_STUDIO_PORT', '18195'))),
                    dispatch, os.environ.get('IMAGE_STUDIO_DISPATCHER_TOKEN', ''))
    dispatch.start()
    try:
        server.serve_forever()
    finally:
        dispatch.stop()
        server.server_close()
