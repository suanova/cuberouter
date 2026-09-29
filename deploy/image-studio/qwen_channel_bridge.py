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

"""Private OpenAI image adapter for the POC /generate worker; RAM only.

Existing OpenAI-compatible image providers do not need this bridge. Configure
this URL and its key in CubeRouter Channel Management for the custom GPU worker.
"""
import base64
import hmac
import json
import os
import time
from email.parser import BytesParser
from email.policy import default
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError
from urllib.request import Request

from dispatcher import HTTP, Server, validate
from queue_core import Rejected


def to_worker(path, content_type, body, model_name):
    if path not in ('/v1/images/generations', '/v1/images/edits'):
        raise Rejected(404, 'unknown_route')
    images = []
    if content_type.startswith('multipart/form-data') and path.endswith('/edits'):
        message = BytesParser(policy=default).parsebytes(
            ('Content-Type: ' + content_type + '\r\nMIME-Version: 1.0\r\n\r\n').encode() + body)
        fields = {}
        if not message.is_multipart():
            raise Rejected(400, 'invalid_multipart')
        for part in message.iter_parts():
            name = part.get_param('name', header='content-disposition')
            data = part.get_payload(decode=True)
            if name in ('image', 'image[]'):
                images.append(base64.b64encode(data).decode('ascii'))
            elif name and name not in fields:
                fields[name] = data.decode('utf-8')
            else:
                raise Rejected(400, 'invalid_fields')
        fields['extra_fields'] = json.loads(fields.get('extra_fields', '{}'))
    elif content_type.split(';')[0] == 'application/json' and path.endswith('/generations'):
        fields = json.loads(body)
    else:
        raise Rejected(415, 'unsupported_content_type')
    if fields.get('model') != model_name or int(fields.get('n', 1)) != 1 or fields.get('response_format') != 'b64_json':
        raise Rejected(400, 'unsupported_image_request')
    size = fields['size'].split('x')
    if len(size) != 2:
        raise Rejected(400, 'invalid_size')
    extra = fields.get('extra_fields', {})
    return validate({'model': '2.1', 'mode': 'edit' if path.endswith('/edits') else 'create',
                     'prompt': fields['prompt'], 'width': int(size[0]), 'height': int(size[1]),
                     'steps': extra.get('steps', 40), 'seed': extra.get('seed', 42),
                     'cfg': extra.get('cfg', 1), 'images': images})


class Bridge(Server):
    daemon_threads = True

    def __init__(self, address, worker_url, token, model_name='qwen-image-2.1'):
        if len(token) < 32:
            raise ValueError('QWEN_BRIDGE_TOKEN must contain at least 32 characters')
        self.worker_url, self.token, self.model_name = worker_url.rstrip('/'), token, model_name
        # Reuse the dispatcher's bounded connection admission, without its
        # queue routes; this bridge only serves authenticated provider calls.
        import threading
        self.slots = threading.BoundedSemaphore(24)
        ThreadingHTTPServer.__init__(self, address, Handler)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, status, data):
        body = json.dumps(data, allow_nan=False).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Cache-Control', 'no-store')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        try:
            self.wfile.write(body)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def do_GET(self):
        if self.path != '/health':
            return self.reply(404, {'error': 'unknown_route'})
        if not hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + self.server.token):
            return self.reply(401, {'error': 'invalid_api_key'})
        try:
            with HTTP.open(self.server.worker_url + '/health', timeout=2) as response:
                body = response.read(16385)
            if len(body) > 16384:
                raise ValueError('health_response_limit')
            data = json.loads(body)
            if not isinstance(data, dict):
                raise ValueError('invalid_health_response')
            self.reply(200, {k: data.get(k) for k in ('state', 'model', 'gpu_count', 'physical_gpus', 'completed')})
        except (OSError, ValueError):
            self.reply(503, {'state': 'unavailable'})

    def do_POST(self):
        try:
            self.connection.settimeout(30)
            if not hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + self.server.token):
                raise Rejected(401, 'invalid_api_key')
            length = int(self.headers.get('Content-Length', '0'))
            if self.headers.get('Transfer-Encoding') or not 0 < length <= 16*1024*1024:
                raise Rejected(413, 'body_limit')
            body = self.rfile.read(length)
            if len(body) != length:
                raise Rejected(400, 'incomplete_body')
            parameters = to_worker(self.path, self.headers.get('Content-Type', ''), body, self.server.model_name)
            request = Request(self.server.worker_url + '/generate', data=json.dumps(parameters, allow_nan=False).encode(),
                              headers={'Content-Type': 'application/json'})
            with HTTP.open(request, timeout=1450) as response:
                data = response.read(24*1024*1024+1)
            if len(data) > 24*1024*1024:
                raise Rejected(502, 'result_limit')
            result = json.loads(data)
            image = result['image']
            if not base64.b64decode(image, validate=True).startswith(b'\x89PNG\r\n\x1a\n'):
                raise Rejected(502, 'invalid_image_result')
            self.reply(200, {'created': int(time.time()), 'data': [{'b64_json': image}]})
        except Rejected as exc:
            self.reply(exc.status, {'error': {'message': exc.code, 'type': 'image_bridge_error'}})
        except HTTPError as exc:
            self.reply(exc.code, {'error': {'message': 'worker_rejected', 'type': 'image_bridge_error'}})
        except (OSError, TimeoutError):
            self.reply(504, {'error': {'message': 'execution_uncertain_no_retry', 'type': 'image_bridge_error'}})
        except (ValueError, TypeError, KeyError):
            self.reply(400, {'error': {'message': 'invalid_request', 'type': 'image_bridge_error'}})


if __name__ == '__main__':
    server = Bridge((os.environ.get('QWEN_BRIDGE_BIND', '127.0.0.1'), int(os.environ.get('QWEN_BRIDGE_PORT', '18423'))),
                    os.environ['QWEN_WORKER_URL'], os.environ.get('QWEN_BRIDGE_TOKEN', ''),
                    os.environ.get('QWEN_BRIDGE_MODEL', 'qwen-image-2.1'))
    try:
        server.serve_forever()
    finally:
        server.server_close()
