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
"""Fault recovery must restore capacity without replaying uncertain inference."""
import io
import json
import socket
import threading
import unittest
from collections import Counter
from email.message import Message
from unittest.mock import patch
from urllib.error import HTTPError, URLError

from dispatcher import channel_reconciler, channel_runner
from queue_core import Dispatcher, Rejected, UncertainExecution
from test_queue import PNG, params, wait_for


class RecoveryTests(unittest.TestCase):
    def test_eight_lost_callbacks_recover_and_pending_job_runs_once(self):
        lose_response, settled = threading.Event(), threading.Event()
        calls, probes = [], []
        def execute(channel, payload):
            calls.append(channel['job_id'])
            if json.loads(payload)['prompt'] == 'lose-response':
                if not lose_response.wait(5): raise RuntimeError('test gate timeout')
                raise UncertainExecution()
            return PNG, {}
        def reconcile(identity):
            probes.append(identity)
            return settled.is_set()
        queue = Dispatcher([{'id': str(i), 'model': '2.1'} for i in range(8)], execute,
                           reconcile=reconcile, recovery_interval=.01, recovery_max_interval=.02)
        self.addCleanup(queue.stop)
        self.addCleanup(lose_response.set)
        queue.start()
        jobs = [queue.submit('cuberouter-'+str(i+1), 'key', params('2.1', 'lose-response'))[0] for i in range(8)]
        wait_for(lambda: len(calls) == 8)
        pending, _ = queue.submit('cuberouter-9', 'pending', params('2.1'))
        lose_response.set()
        wait_for(lambda: queue.health()['quarantined_slots'] == 8 and len(probes) >= 8)
        self.assertEqual(queue.health()['status'], 'unavailable')
        self.assertEqual(queue.status('cuberouter-9', pending['id'])['state'], 'queued')
        self.assertEqual(len(calls), 8)
        with self.assertRaises(Rejected) as rejected:
            queue.submit('cuberouter-10', 'new', params('2.1'))
        self.assertEqual(rejected.exception.code, 'execution_capacity_unavailable')
        # Repeating a request key must return its failed identity, even during outage.
        existing, created = queue.submit('cuberouter-1', 'key', params('2.1', 'lose-response'))
        self.assertFalse(created)
        self.assertEqual(existing['id'], jobs[0]['id'])
        settled.set()
        wait_for(lambda: queue.health()['status'] == 'ready')
        wait_for(lambda: queue.status('cuberouter-9', pending['id'])['state'] == 'completed')
        self.assertEqual(Counter(calls), Counter([j['id'] for j in jobs] + [pending['id']]))
        self.assertTrue(all(set(p) == {'owner', 'job_id'} for p in probes))
        for i, job in enumerate(jobs):
            self.assertEqual(queue.status('cuberouter-'+str(i+1), job['id'])['state'], 'failed')
        queue.ack('cuberouter-9', pending['id'])
        self.assertEqual(queue.metrics()['content_bytes'], 0)

    def test_recovery_survives_metadata_expiry_and_does_not_hold_queue_lock(self):
        probing, release, recovered = threading.Event(), threading.Event(), threading.Event()
        calls = []
        def execute(c, p): raise UncertainExecution()
        def reconcile(identity):
            calls.append(identity)
            probing.set()
            release.wait(5)
            return recovered.is_set()
        queue = Dispatcher([{'id': 'one', 'model': '2.1'}], execute, reconcile=reconcile,
                           metadata_ttl=.01, recovery_interval=.01, recovery_max_interval=.02)
        self.addCleanup(queue.stop)
        self.addCleanup(release.set)
        queue.start()
        job, _ = queue.submit('cuberouter-1', 'key', params('2.1'))
        self.assertTrue(probing.wait(2))
        observed = threading.Event()
        threading.Thread(target=lambda: (queue.metrics(), observed.set()), daemon=True).start()
        self.assertTrue(observed.wait(1), 'network probe held the queue condition lock')
        wait_for(lambda: queue.metrics()['retained_jobs'] == 0)
        recovered.set(); release.set()
        # No new work is needed to drive recovery; expired jobs aren't dereferenced.
        wait_for(lambda: queue.health()['status'] == 'ready')
        self.assertEqual(calls[0], {'owner': 'cuberouter-1', 'job_id': job['id']})
        self.assertNotIn('recovery', queue.channels[0])

    def test_inconclusive_probes_hold_slot_then_eventually_recover(self):
        probes, allow = [], threading.Event()
        def execute(c, p): raise UncertainExecution()
        def reconcile(identity):
            probes.append(identity)
            if len(probes) == 1: raise TimeoutError()
            return True if allow.is_set() else None
        queue = Dispatcher([{'id': 'one', 'model': '2.1'}], execute, reconcile=reconcile,
                           recovery_interval=.01, recovery_max_interval=.02)
        self.addCleanup(queue.stop); queue.start()
        queue.submit('cuberouter-1', 'key', params('2.1'))
        wait_for(lambda: len(probes) >= 2)
        self.assertEqual(queue.health()['status'], 'unavailable')
        allow.set()
        wait_for(lambda: queue.health()['status'] == 'ready')

    def test_owner_cannot_start_another_job_while_execution_is_uncertain(self):
        settled = threading.Event()
        def execute(c, payload):
            if json.loads(payload)['prompt'] == 'uncertain': raise UncertainExecution()
            return PNG, {}
        queue = Dispatcher([{'id': str(i), 'model': '2.1'} for i in range(2)], execute,
                           reconcile=lambda identity: settled.is_set(), metadata_ttl=.01,
                           recovery_interval=.01, recovery_max_interval=.02)
        self.addCleanup(queue.stop); queue.start()
        failed, _ = queue.submit('cuberouter-1', 'first', params('2.1', 'uncertain'))
        wait_for(lambda: queue.health()['status'] == 'degraded')
        for metadata_expired in (False, True):
            if metadata_expired: wait_for(lambda: queue.metrics()['retained_jobs'] == 0)
            with self.assertRaises(Rejected) as rejected:
                queue.submit('cuberouter-1', 'another', params('2.1'))
            self.assertEqual(rejected.exception.code, 'execution_recovery_pending')
        other, _ = queue.submit('cuberouter-2', 'other', params('2.1'))
        wait_for(lambda: queue.status('cuberouter-2', other['id'])['state'] == 'completed')
        settled.set()
        wait_for(lambda: queue.health()['status'] == 'ready')
        new, _ = queue.submit('cuberouter-1', 'after-recovery', params('2.1'))
        wait_for(lambda: queue.status('cuberouter-1', new['id'])['state'] == 'completed')
        self.assertNotEqual(new['id'], failed['id'])

    def test_idle_unavailable_direct_worker_can_recover_after_queue_expiry(self):
        online = threading.Event()
        queue = Dispatcher([{'id': 'one', 'model': '2.1'}], lambda c,p: (PNG,{}),
                           ready=lambda c: online.is_set(), queue_ttl=.01)
        self.addCleanup(queue.stop); queue.start()
        job, _ = queue.submit('cuberouter-1', 'key', params('2.1'))
        wait_for(lambda: queue.status('cuberouter-1', job['id'])['state'] == 'expired')
        self.assertEqual(queue.health()['status'], 'unavailable')
        online.set()
        wait_for(lambda: queue.health()['status'] == 'ready')


class ReconcileProtocolTests(unittest.TestCase):
    identity = {'owner': 'cuberouter-1', 'job_id': 'a'*32}
    def response(self, body, content_type='application/json', status=200):
        response = io.BytesIO(body if isinstance(body, bytes) else json.dumps(body).encode())
        response.status = status
        response.headers = Message()
        response.headers['Content-Type'] = content_type
        return response

    def valid(self, **changes):
        return {'protocol': 'image-studio-recovery-v1', 'epoch': 'b'*32,
                **self.identity, 'state': 'settled', **changes}

    def test_releases_only_matching_versioned_terminal_reply(self):
        reconcile = channel_reconciler('http://private', 's'*40)
        for state in ('settled', 'fenced', 'active', 'unknown'):
            with self.subTest(state=state), patch('dispatcher.HTTP.open', return_value=self.response(self.valid(state=state))) as opened:
                self.assertEqual(reconcile(self.identity), state in ('settled', 'fenced'))
                request = opened.call_args.args[0]
                self.assertEqual(request.full_url, 'http://private/internal/image-studio/reconcile')
                self.assertEqual(request.get_header('X-image-owner'), self.identity['owner'])
                self.assertEqual(request.get_header('X-image-job'), self.identity['job_id'])
                self.assertEqual(request.data, b'{}')
                self.assertEqual(opened.call_args.kwargs['timeout'], 3)
        # After a single-gateway restart its new epoch may atomically fence an old ID.
        with patch('dispatcher.HTTP.open', return_value=self.response(self.valid(state='fenced', epoch='c'*32))):
            self.assertTrue(reconcile(self.identity))

    def test_bad_reply_never_releases_capacity(self):
        cases = [self.valid(owner='cuberouter-2'), self.valid(job_id='d'*32),
                 self.valid(protocol='old'), self.valid(epoch=''), {}, [],
                 b'<html>SPA catch-all</html>', b'x'*4097]
        reconcile = channel_reconciler('http://private', 's'*40)
        for value in cases:
            with self.subTest(value=str(value)[:100]), patch('dispatcher.HTTP.open', return_value=self.response(value)):
                self.assertFalse(reconcile(self.identity))
        for content_type, status in [('text/html', 200), ('application/json', 202)]:
            with patch('dispatcher.HTTP.open', return_value=self.response(self.valid(), content_type, status)):
                self.assertFalse(reconcile(self.identity))
        for exc in (TimeoutError(), URLError('offline'), HTTPError('http://private', 503, 'down', {}, None)):
            with patch('dispatcher.HTTP.open', side_effect=exc):
                self.assertFalse(reconcile(self.identity))

    def test_only_definite_preconnection_failure_avoids_quarantine(self):
        execute = channel_runner('http://private', 's'*40)
        for reason in (ConnectionRefusedError(), socket.gaierror()):
            with patch('dispatcher.HTTP.open', side_effect=URLError(reason)), self.assertRaisesRegex(RuntimeError, 'relay_not_accepted'):
                execute(self.identity, b'{}')
        for exc in (URLError(TimeoutError()), ConnectionResetError(),
                    HTTPError('http://private', 503, 'unknown acceptance', {}, None),
                    HTTPError('http://private', 429, 'unknown acceptance', {}, None)):
            with patch('dispatcher.HTTP.open', side_effect=exc), self.assertRaises(UncertainExecution):
                execute(self.identity, b'{}')


if __name__ == '__main__':
    unittest.main()
