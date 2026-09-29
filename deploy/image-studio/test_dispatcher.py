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

"""HTTP trust boundary and single-GPU mixed create/edit regression tests."""
import base64
import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import Request, build_opener, ProxyHandler
from urllib.error import HTTPError
from dispatcher import Server, validate, run
from queue_core import Dispatcher, Rejected
from test_queue import PNG, wait_for


def payload(mode='create'):
    return dict(model='2.1', mode=mode, prompt='test image', width=512, height=512,
                steps=2, cfg=1, seed=42, images=[] if mode == 'create' else [base64.b64encode(PNG).decode()])


class HTTPTests(unittest.TestCase):
    def setUp(self):
        self.gate = threading.Event()
        self.calls = []
        def runner(channel, data):
            self.calls.append(json.loads(data)['mode'])
            if not self.gate.wait(5):
                raise RuntimeError('test timeout')
            return PNG, {'model': '2.1'}
        self.dispatch = Dispatcher([dict(id='gpu-0', model='2.1')], runner)
        self.server = Server(('127.0.0.1', 0), self.dispatch, 'test-token-' * 4)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.dispatch.start()

    def tearDown(self):
        self.gate.set()
        self.server.shutdown()
        self.server.server_close()
        self.dispatch.stop()
        self.thread.join()

    def request(self, path, data=None, owner='cuberouter-1', token=None, epoch=None, key='test-request-000001'):
        headers = {'Authorization': 'Bearer ' + (self.server.token if token is None else token),
                   'X-Image-Owner': owner, 'X-Trial-Epoch': epoch or self.server.epoch,
                   'Idempotency-Key': key, 'Content-Type': 'application/json'}
        req = Request('http://127.0.0.1:%s%s' % (self.server.server_port, path),
                      data=json.dumps(data).encode() if data is not None else None, headers=headers)
        try:
            response = build_opener(ProxyHandler({})).open(req)
        except HTTPError as e:
            response = e
        with response:
            raw = response.read()
            self.assertEqual(response.headers['Cache-Control'], 'no-store')
            return response.status, raw if response.headers['Content-Type'] == 'image/png' else json.loads(raw)

    def test_auth_epoch_and_identity_required(self):
        self.assertEqual(self.request('/session', token='wrong')[0], 401)
        self.assertEqual(self.request('/session', owner='anonymous')[0], 403)
        self.assertEqual(self.request('/jobs', payload(), epoch='old')[0], 409)
        self.assertEqual(self.request('/jobs', payload(), key='short')[0], 400)
        self.assertEqual(self.request('/session')[1]['owner'], 'cuberouter-1')
        self.assertEqual(self.dispatch.metrics()['retained_jobs'], 0)

    def test_four_users_share_one_fifo_and_ack_erases_content(self):
        jobs = []
        for i, mode in enumerate(['create', 'edit', 'create', 'edit'], 1):
            owner = 'cuberouter-%s' % i
            code, job = self.request('/jobs', payload(mode), owner)
            self.assertEqual(code, 202)
            jobs.append((owner, job['id']))
            if i == 1:
                wait_for(lambda: self.dispatch.status(owner, job['id'])['state'] == 'running')
        self.assertEqual(self.dispatch.metrics()['models']['2.1']['running'], 1)
        self.assertEqual(self.dispatch.metrics()['models']['2.1']['queued'], 3)
        self.assertEqual(self.request('/jobs', payload())[0], 200)  # retry same request
        self.assertEqual(self.request('/requests/test-request-000001')[1]['id'], jobs[0][1])
        self.assertEqual(self.request('/requests/test-request-000001', owner='cuberouter-9')[0], 404)
        self.assertEqual(self.request('/jobs', payload(), key='another-request-0001')[0], 429)
        for suffix, body in [('', None), ('/result', None), ('/ack', {}), ('/cancel', {})]:
            self.assertEqual(self.request('/jobs/' + jobs[0][1] + suffix, body, 'cuberouter-9')[0], 404)
        self.gate.set()
        wait_for(lambda: all(self.dispatch.status(o, j)['state'] == 'completed' for o, j in jobs))
        self.assertEqual(self.calls, ['create', 'edit', 'create', 'edit'])
        for owner, ident in jobs:
            self.assertEqual(self.request('/jobs/' + ident + '/result', owner=owner)[1], PNG)
            self.assertEqual(self.request('/jobs/' + ident + '/ack', {}, owner)[0], 200)
            self.assertEqual(self.request('/jobs/' + ident + '/result', owner=owner)[0], 410)
        self.assertEqual(self.dispatch.metrics()['content_bytes'], 0)

    def test_bad_payloads_never_reach_worker(self):
        cases = [dict(mode='edit'), dict(model='2512'), dict(steps=True), dict(width=513),
                 dict(images=['bad-base64']), dict(cfg=float('inf')), dict(unknown='value')]
        for change in cases:
            with self.subTest(change=change):
                self.assertEqual(self.request('/jobs', {**payload(), **change})[0], 400)
        self.assertEqual(self.calls, [])

    def test_unicode_prompt_and_ten_reference_order_reach_worker_unchanged(self):
        received = []
        class Worker(BaseHTTPRequestHandler):
            def log_message(self, *args): pass
            def do_POST(self):
                received.append((self.path, json.loads(self.rfile.read(int(self.headers['Content-Length'])))))
                data = json.dumps({'image': base64.b64encode(PNG).decode()}).encode()
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)
        worker = ThreadingHTTPServer(('127.0.0.1', 0), Worker)
        thread = threading.Thread(target=worker.serve_forever, daemon=True)
        thread.start()
        try:
            self.dispatch.runner = run
            self.dispatch.channels[0]['url'] = f'http://127.0.0.1:{worker.server_port}'
            data = payload('edit')
            data['prompt'] = '將花變成藍色，加入「中秋快樂」與 "Hello"。\n保留 {{原文}}、\\、🌕。\n' + '尾段不可遺失' * 1000
            data['images'] = [base64.b64encode(PNG+str(i).encode()).decode() for i in range(10)]
            code, job = self.request('/jobs', data)
            self.assertEqual(code, 202)
            wait_for(lambda:self.dispatch.status('cuberouter-1', job['id'])['state']=='completed')
            self.assertEqual(received, [('/generate', data)])
            self.assertEqual(self.request('/jobs/'+job['id']+'/ack', {})[0], 200)
            self.assertEqual(self.dispatch.metrics()['content_bytes'], 0)
            code, _ = self.request('/jobs', {**data, 'images': data['images'] + data['images'][:1]}, key='eleven-images-test')
            self.assertEqual(code, 400)
            self.assertEqual(len(received), 1)
        finally:
            worker.shutdown(); worker.server_close(); thread.join()


if __name__ == '__main__':
    unittest.main()
