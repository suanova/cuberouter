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
"""Bounded, RAM-only FIFO for image review. One serial consumer per replica.

Run ONE pool process. A replica must belong to only one pool. This service does
not route generation, store images, or authorize releasing an image itself.
CubeRouter still validates the returned hash, policy scores and model revision.
"""
import collections
import hmac
import json
import os
from pathlib import Path
import ssl
import threading
import time
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import Request, ProxyHandler, HTTPSHandler, HTTPRedirectHandler, build_opener
from urllib.parse import urlparse

MAX_IMAGE = 12 * 1024 * 1024
ERROR = b'{"decision":"error","release_allowed":false}'


@dataclass
class Job:
    data: bytes
    deadline: float
    tried: set = field(default_factory=set)
    done: threading.Event = field(default_factory=threading.Event)
    result: bytes = ERROR
    status: int = 503


class ReviewPool:
    def __init__(self, replicas, call, capacity=64, byte_limit=256*1024*1024, wait=35):
        if not 1 <= len(replicas) <= 16 or not 1 <= capacity <= 128 or not 1 <= wait <= 35:
            raise ValueError('Invalid bounded pool configuration')
        self.replicas, self.call = replicas, call
        self.capacity, self.byte_limit, self.wait = capacity, byte_limit, wait
        self.cv = threading.Condition()
        self.pending = collections.deque()
        self.count = self.bytes = self.active = 0
        self.passed = self.failed = 0
        self.stopped = False
        self.threads = [threading.Thread(target=self.consume, args=(i,), daemon=True)
                        for i in range(len(replicas))]
        for thread in self.threads:
            thread.start()

    def submit(self, data):
        with self.cv:
            if self.stopped or self.count >= self.capacity or self.bytes + len(data) > self.byte_limit:
                return None
            job = Job(data, time.monotonic() + self.wait)
            self.count += 1
            self.bytes += len(data)
            self.pending.append(job)
            self.cv.notify_all()
            return job

    def finish(self, job, result=ERROR, status=503):
        # Caller holds cv; never release byte accounting while inference runs.
        self.bytes -= len(job.data)
        job.data = b''
        self.count -= 1
        self.passed += status == 200
        self.failed += status != 200
        job.result, job.status = result, status
        job.done.set()
        self.cv.notify_all()

    def consume(self, index):
        while True:
            with self.cv:
                while not self.pending and not self.stopped:
                    self.cv.wait()
                if self.stopped:
                    return
                # Expired jobs never reach a classifier. Retry may move a failed
                # job behind newer work, but never back onto the failed replica.
                for job in list(self.pending):
                    if time.monotonic() >= job.deadline:
                        self.pending.remove(job)
                        self.finish(job)
                job = next((j for j in self.pending if index not in j.tried), None)
                if job is None:
                    self.cv.wait(0.05)
                    continue
                self.pending.remove(job)
                job.tried.add(index)
                self.active += 1
            failed = False
            try:
                result = self.call(self.replicas[index], job.data, max(0.1, min(10, job.deadline-time.monotonic())))
                if not isinstance(result, bytes) or len(result) > 16384:
                    raise ValueError('Invalid review response')
                with self.cv:
                    self.active -= 1
                    if time.monotonic() < job.deadline:
                        self.finish(job, result, 200)
                    else:
                        self.finish(job)
            except Exception:
                failed = True
                with self.cv:
                    self.active -= 1
                    if len(job.tried) < len(self.replicas) and time.monotonic() < job.deadline and not self.stopped:
                        self.pending.appendleft(job)
                        self.cv.notify_all()
                    else:
                        self.finish(job)
            if failed:
                # Avoid repeatedly hammering an unavailable replica. Other
                # consumers continue working, including this job's safe retry.
                with self.cv:
                    self.cv.wait(2)

    def snapshot(self):
        with self.cv:
            return {'replicas': len(self.replicas), 'active': self.active,
                    'queued': len(self.pending), 'admitted': self.count,
                    'bytes': self.bytes, 'capacity': self.capacity,
                    'completed': self.passed, 'failed': self.failed}

    def close(self):
        with self.cv:
            self.stopped = True
            while self.pending:
                self.finish(self.pending.popleft())
            self.cv.notify_all()
        for thread in self.threads:
            thread.join(12)


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def load_replicas(path):
    result = []
    for item in json.loads(Path(path).read_text())['replicas']:
        url = urlparse(item['url'])
        loopback = url.hostname in ('127.0.0.1', '::1', 'localhost')
        if (url.scheme != 'https' and not (url.scheme == 'http' and loopback)) or url.username or url.query or url.fragment:
            raise ValueError('Review endpoints require TLS or loopback')
        if not url.hostname or url.path != '/v1/moderate':
            raise ValueError('Invalid review endpoint')
        token = Path(item['token_file']).read_text().strip()
        if not 32 <= len(token) <= 256 or not token.isascii() or any(c.isspace() for c in token):
            raise ValueError('Invalid replica token')
        context = ssl.create_default_context(cafile=item.get('ca_file'))
        result.append({'url': item['url'], 'token': token,
                       'http': build_opener(ProxyHandler({}), HTTPSHandler(context=context), NoRedirect())})
    if len({x['url'] for x in result}) != len(result):
        raise ValueError('Duplicate review replica')
    return result


def classify(replica, data, timeout):
    request = Request(replica['url'], data=data, headers={
        'Content-Type': 'image/png', 'Authorization': 'Bearer '+replica['token']})
    with replica['http'].open(request, timeout=timeout) as response:
        if response.status != 200 or response.headers.get_content_type() != 'application/json':
            raise ValueError('Review unavailable')
        data = response.read(16385)
    if len(data) > 16384:
        raise ValueError('Review response too large')
    return data


class PoolServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, address, pool, token):
        self.pool, self.token = pool, token
        # Bound uploads as well as admitted images; body reads have a timeout.
        self.connections = threading.BoundedSemaphore(72)
        super().__init__(address, Handler)

    def process_request(self, request, address):
        if not self.connections.acquire(blocking=False):
            self.shutdown_request(request)
            return
        try:
            super().process_request(request, address)
        except Exception:
            self.connections.release()
            raise

    def process_request_thread(self, request, address):
        try:
            super().process_request_thread(request, address)
        finally:
            self.connections.release()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, status, data):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Cache-Control', 'no-store')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        try:
            self.wfile.write(data)
        except (BrokenPipeError, ConnectionResetError):
            pass

    def authorized(self):
        return hmac.compare_digest(self.headers.get('Authorization', '').encode(),
                                   ('Bearer '+self.server.token).encode())

    def do_GET(self):
        if not self.authorized():
            return self.reply(401, ERROR)
        if self.path != '/v1/health':
            return self.reply(404, ERROR)
        # Pool liveness and occupancy, not a promise that every GPU is healthy.
        return self.reply(200, json.dumps(self.server.pool.snapshot()).encode())

    def do_POST(self):
        self.connection.settimeout(10)
        if not self.authorized():
            return self.reply(401, ERROR)
        if self.path != '/v1/moderate':
            return self.reply(404, ERROR)
        try:
            size = int(self.headers.get('Content-Length', '0'))
            if self.headers.get('Transfer-Encoding') or not 0 < size <= MAX_IMAGE:
                return self.reply(413, ERROR)
            if self.headers.get('Content-Type', '').split(';')[0] != 'image/png':
                return self.reply(415, ERROR)
            data = self.rfile.read(size)
            if len(data) != size or not data.startswith(b'\x89PNG\r\n\x1a\n'):
                return self.reply(400, ERROR)
            job = self.server.pool.submit(data)
            data = None
            if job is None:
                return self.reply(503, ERROR)
            if not job.done.wait(self.server.pool.wait + 0.5):
                return self.reply(503, ERROR)
            self.reply(job.status, job.result)
        except (ValueError, OSError):
            self.reply(400, ERROR)


if __name__ == '__main__':
    token = Path(os.environ['MODERATION_POOL_TOKEN_FILE']).read_text().strip()
    if not 32 <= len(token) <= 256 or not token.isascii() or any(c.isspace() for c in token):
        raise ValueError('Invalid pool token')
    pool = ReviewPool(load_replicas(os.environ['MODERATION_POOL_REPLICAS_FILE']), classify)
    server = PoolServer((os.environ.get('MODERATION_POOL_BIND', '127.0.0.1'),
                         int(os.environ.get('MODERATION_POOL_PORT', '18461'))), pool, token)
    cert, key = os.environ.get('MODERATION_POOL_TLS_CERT'), os.environ.get('MODERATION_POOL_TLS_KEY')
    if cert and key:
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.minimum_version = ssl.TLSVersion.TLSv1_2
        context.load_cert_chain(cert, key)
        server.socket = context.wrap_socket(server.socket, server_side=True)
    elif server.server_address[0] not in ('127.0.0.1', '::1'):
        raise ValueError('Non-loopback pool must use TLS')
    try:
        server.serve_forever()
    finally:
        server.server_close()
        pool.close()
