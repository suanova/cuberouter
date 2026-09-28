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

import base64
import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError
from urllib.request import Request

from dispatcher import HTTP, channel_runner
from qwen_channel_bridge import Bridge, to_worker
from queue_core import Dispatcher
from test_queue import wait_for, PNG


class ChannelTests(unittest.TestCase):
    def test_health_requires_key_and_returns_only_bounded_worker_status(self):
        class Worker(BaseHTTPRequestHandler):
            calls = 0
            value = {'state': 'ready', 'model': '2.1', 'gpu_count': 1,
                     'physical_gpus': [3], 'completed': 7, 'private_detail': 'omit'}
            def log_message(self, *args): pass
            def do_GET(self):
                Worker.calls += 1
                self.send_response(200)
                self.end_headers()
                self.wfile.write(json.dumps(Worker.value).encode())
        worker = ThreadingHTTPServer(('127.0.0.1', 0), Worker)
        bridge = Bridge(('127.0.0.1', 0), f'http://127.0.0.1:{worker.server_port}', 'k' * 40)
        for server in (worker, bridge):
            self.addCleanup(server.server_close)
            self.addCleanup(server.shutdown)
            threading.Thread(target=server.serve_forever, daemon=True).start()
        url = f'http://127.0.0.1:{bridge.server_port}/health'
        with self.assertRaises(HTTPError) as denied:
            HTTP.open(url, timeout=3)
        self.assertEqual(denied.exception.code, 401)
        denied.exception.close()
        self.assertEqual(Worker.calls, 0)
        request = Request(url, headers={'Authorization': 'Bearer ' + 'k' * 40})
        with HTTP.open(request, timeout=3) as response:
            value = json.loads(response.read())
        self.assertEqual(value, {k: v for k, v in Worker.value.items() if k != 'private_detail'})
        for malformed in ([], {'extra': 'x' * 17000}):
            Worker.value = malformed
            with self.subTest(malformed=type(malformed).__name__), self.assertRaises(HTTPError) as failed:
                HTTP.open(request, timeout=3)
            self.assertEqual(failed.exception.code, 503)
            failed.exception.close()

    def test_bridge_preserves_parameters(self):
        p = to_worker('/v1/images/generations', 'application/json', json.dumps({
            'model': 'qwen-image-2.1', 'prompt': '藍色位置加花 🌸\n保留原圖', 'n': 1,
            'size': '512x512', 'response_format': 'b64_json',
            'extra_fields': {'steps': 4, 'seed': 123, 'cfg': 1}}).encode(), 'qwen-image-2.1')
        self.assertEqual(p['prompt'], '藍色位置加花 🌸\n保留原圖')
        self.assertEqual((p['steps'], p['seed'], p['cfg']), (4, 123, 1))

    def test_queue_callback_identity_and_no_duplicate_execution(self):
        seen = []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_POST(self):
                body = self.rfile.read(int(self.headers['Content-Length']))
                seen.append((self.path, dict(self.headers), body))
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('X-Image-Studio-Channel', '23')
                self.end_headers()
                self.wfile.write(json.dumps({'data': [{'b64_json': base64.b64encode(PNG).decode()}]}).encode())
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        runner = channel_runner('http://127.0.0.1:' + str(server.server_port), 's'*40)
        queue = Dispatcher([{'id': 'slot-1', 'model': '2.1'}], runner)
        self.addCleanup(queue.stop)
        queue.start()
        p = {'model': '2.1', 'prompt': 'Keep exact prompt'}
        job, _ = queue.submit('cuberouter-71', 'request-12345678', p)
        queue.submit('cuberouter-71', 'request-12345678', p)
        wait_for(lambda: queue.status('cuberouter-71', job['id'])['state'] == 'completed')
        self.assertEqual(len(seen), 1)
        path, headers, body = seen[0]
        self.assertEqual(path, '/internal/image-studio/execute')
        self.assertEqual(headers['X-Image-Owner'], 'cuberouter-71')
        self.assertEqual(headers['X-Image-Job'], job['id'])
        self.assertEqual(headers['Authorization'], 'Bearer ' + 's'*40)
        self.assertEqual(json.loads(body), p)
        self.assertEqual(queue.result('cuberouter-71', job['id']), PNG)
        queue.ack('cuberouter-71', job['id'])
        self.assertEqual(queue.metrics()['content_bytes'], 0)

    def test_relay_failure_never_retries(self):
        class Handler(BaseHTTPRequestHandler):
            calls = 0
            def log_message(self, *args): pass
            def do_POST(self):
                Handler.calls += 1
                self.send_response(503)
                self.end_headers()
        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        runner = channel_runner('http://127.0.0.1:' + str(server.server_port), 's'*40)
        queue = Dispatcher([{'id': 'slot', 'model': '2.1'}], runner)
        self.addCleanup(queue.stop)
        queue.start()
        job, _ = queue.submit('cuberouter-71', 'failure-key', {'model': '2.1'})
        wait_for(lambda: queue.status('cuberouter-71', job['id'])['state'] == 'failed')
        self.assertEqual(Handler.calls, 1)


if __name__ == '__main__':
    unittest.main()
