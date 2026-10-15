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
"""Live channel count changes, draining, and authenticated safe admission retry."""
import base64
import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from dispatcher import channel_capacity, channel_runner
from queue_core import BusyBeforeAccept, Dispatcher, Rejected, UncertainExecution
from test_queue import PNG, params, wait_for


class DynamicChannelTests(unittest.TestCase):
    def test_auto_pool_runs_32_distinct_slots_with_33rd_waiting(self):
        gate = threading.Event()
        lock = threading.Lock()
        active, calls, peak = set(), [], [0]
        def run(channel, payload):
            with lock:
                if channel['id'] in active: raise AssertionError('overlapping slot')
                active.add(channel['id'])
                calls.append(channel['job_id'])
                peak[0] = max(peak[0], len(active))
            if not gate.wait(5): raise RuntimeError('test timeout')
            with lock: active.remove(channel['id'])
            return PNG, {}
        queue = Dispatcher([{'id': str(i), 'model': '2.1'} for i in range(64)], run,
                           capacity_reader=lambda: 32)
        self.addCleanup(queue.stop)
        self.addCleanup(gate.set)
        queue.start()
        jobs = []
        for i in range(33):
            job, _ = queue.submit(str(i), str(i), params('2.1'))
            jobs.append(job['id'])
            if i < 32: wait_for(lambda: queue.status(str(i), job['id'])['state'] == 'running')
        self.assertEqual(queue.health()['total_slots'], 32)
        self.assertEqual(queue.metrics()['models']['2.1']['running'], 32)
        self.assertEqual(queue.status('32', jobs[32])['state'], 'queued')
        gate.set()
        wait_for(lambda: all(queue.status(str(i), job)['state'] == 'completed' for i, job in enumerate(jobs)))
        self.assertEqual(peak[0], 32)
        self.assertCountEqual(calls, jobs)

    def test_live_growth_shrink_pause_and_resume_without_duplicate_generation(self):
        capacity = [1]
        calls = []
        gates = [threading.Event() for _ in range(4)]
        def run(channel, payload):
            index = int(json.loads(payload)['prompt'])
            calls.append((index, channel['job_id']))
            if not gates[index].wait(5):
                raise RuntimeError('test gate timeout')
            return PNG, {}
        queue = Dispatcher([{'id': str(i), 'model': '2.1'} for i in range(4)], run,
                           capacity_reader=lambda: capacity[0], capacity_interval=.02)
        self.addCleanup(queue.stop)
        self.addCleanup(lambda: [g.set() for g in gates])
        queue.start()
        jobs = [queue.submit(str(i), str(i), params('2.1', str(i)))[0]['id'] for i in range(4)]
        wait_for(lambda: len(calls) == 1)
        self.assertEqual(queue.metrics()['models']['2.1']['queued'], 3)
        capacity[0] = 3
        wait_for(lambda: len(calls) == 3)
        capacity[0] = 0
        wait_for(lambda: queue.health()['total_slots'] == 0)
        with self.assertRaises(Rejected) as denied:
            queue.submit('new', 'new', params('2.1'))
        self.assertEqual(denied.exception.status, 503)
        for gate in gates[:3]: gate.set()
        wait_for(lambda: all(queue.status(str(i), jobs[i])['state'] == 'completed' for i in range(3)))
        self.assertEqual(queue.status('3', jobs[3])['state'], 'queued')
        self.assertEqual(len(calls), 3)
        capacity[0] = 1
        gates[3].set()
        wait_for(lambda: queue.status('3', jobs[3])['state'] == 'completed')
        self.assertCountEqual(calls, list(enumerate(jobs)))
        self.assertEqual(queue.health()['total_slots'], 1)

    def test_failed_refresh_pauses_and_disabled_uncertain_slot_still_recovers(self):
        capacity = [1]
        settled = threading.Event()
        def read():
            if capacity[0] is None: raise OSError('offline')
            return capacity[0]
        def run(c, p): raise UncertainExecution()
        queue = Dispatcher([{'id': '0', 'model': '2.1'}], run, capacity_reader=read,
                           capacity_interval=.02, reconcile=lambda _: settled.is_set(),
                           recovery_interval=.01, recovery_max_interval=.02)
        self.addCleanup(queue.stop)
        queue.start()
        job, _ = queue.submit('a', 'a', params('2.1'))
        wait_for(lambda: queue.status('a', job['id'])['state'] == 'failed')
        capacity[0] = None
        wait_for(lambda: queue.health()['total_slots'] == 0)
        settled.set()
        wait_for(lambda: queue.channels[0]['state'] == 'idle')
        for invalid in (True, -1, 2):
            capacity[0] = invalid
            queue._refresh_capacity()
            self.assertEqual(queue.health()['total_slots'], 0)
        queue.runner = lambda c, p: (PNG, {})
        capacity[0] = 1
        wait_for(lambda: queue.health()['status'] == 'ready')
        next_job, _ = queue.submit('a', 'next', params('2.1'))
        wait_for(lambda: queue.status('a', next_job['id'])['state'] == 'completed')

    def server(self, handler):
        server = ThreadingHTTPServer(('127.0.0.1', 0), handler)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        return 'http://127.0.0.1:' + str(server.server_port)

    def test_capacity_protocol_auth_and_bounds(self):
        seen = []
        class Handler(BaseHTTPRequestHandler):
            value = {'protocol': 'image-studio-capacity-v1', 'model': '2.1', 'capacity': 8}
            def log_message(self, *args): pass
            def do_GET(self):
                seen.append((self.path, self.headers['Authorization']))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(json.dumps(self.value).encode())
        read = channel_capacity(self.server(Handler), 't' * 40)
        self.assertEqual(read(), 8)
        self.assertEqual(seen, [('/internal/image-studio/capacity', 'Bearer ' + 't' * 40)])
        valid = dict(Handler.value)
        for value in (0, 1, 64):
            Handler.value = {**valid, 'capacity': value}
            self.assertEqual(read(), value)
        for value in (True, -1, 65, '8', None):
            Handler.value = {**valid, 'capacity': value}
            with self.assertRaises(ValueError): read()
        for value in ([], {**valid, 'protocol': 'wrong'}, {**valid, 'model': 'other'},
                      {**valid, 'padding': 'x' * 4096}):
            Handler.value = value
            with self.assertRaises(ValueError): read()

    def test_busy_retries_same_identity_only_with_explicit_unstarted_reply(self):
        seen = []
        class Handler(BaseHTTPRequestHandler):
            code = 429
            reply = {'protocol': 'image-studio-admission-v1', 'state': 'not_started',
                     'owner': 'cuberouter-71', 'job_id': 'a' * 32, 'error': 'image_channel_busy'}
            def log_message(self, *args): pass
            def do_POST(self):
                self.rfile.read(int(self.headers['Content-Length']))
                seen.append((self.headers['X-Image-Owner'], self.headers['X-Image-Job']))
                self.send_response(self.code)
                self.end_headers()
                self.wfile.write(json.dumps(self.reply).encode())
        run = channel_runner(self.server(Handler), 't' * 40)
        channel = {'owner': 'cuberouter-71', 'job_id': 'a' * 32}
        valid = dict(Handler.reply)
        with self.assertRaises(BusyBeforeAccept): run(channel, b'{}')
        for field, wrong in [('protocol', 'wrong'), ('state', 'active'), ('owner', 'cuberouter-72'),
                             ('job_id', 'b' * 32), ('error', 'other')]:
            Handler.reply = {**valid, field: wrong}
            with self.subTest(field=field), self.assertRaises(UncertainExecution): run(channel, b'{}')
        Handler.reply = valid
        Handler.code = 503
        with self.assertRaises(UncertainExecution): run(channel, b'{}')
        Handler.code = 200
        Handler.reply = {'data': [{'b64_json': base64.b64encode(PNG).decode()}]}
        self.assertEqual(run(channel, b'{}')[0], PNG)
        self.assertTrue(all(item == ('cuberouter-71', 'a' * 32) for item in seen))


if __name__ == '__main__': unittest.main()
