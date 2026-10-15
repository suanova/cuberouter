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
import io
import json
import unittest
from unittest.mock import patch
from urllib.error import HTTPError

from dispatcher import channel_runner
from queue_core import Dispatcher, OutputRejected, Rejected, UncertainExecution
from test_queue import PNG, wait_for


class OutputReviewTests(unittest.TestCase):
    def test_only_identity_bound_terminal_decisions_release_slot_without_retry(self):
        channel = {'owner': 'cuberouter-71', 'job_id': 'a' * 32, 'execution_deadline': 2000000000}
        runner = channel_runner('http://127.0.0.1:3000', 'r' * 40)
        for error in ['output_blocked', 'output_check_unavailable']:
            data = {'protocol': 'image-studio-output-v1', 'job_id': channel['job_id'],
                    'owner': channel['owner'], 'error': error}
            with patch('dispatcher.HTTP.open', side_effect=HTTPError('', 422, '', {}, io.BytesIO(json.dumps(data).encode()))) as call:
                with self.assertRaises(OutputRejected) as result:
                    runner(channel, b'{}')
                self.assertEqual(result.exception.code, error)
                self.assertEqual(call.call_count, 1)
            for field in ['job_id', 'owner', 'protocol', 'error']:
                invalid = {**data, field: 'wrong'}
                with patch('dispatcher.HTTP.open', side_effect=HTTPError('', 422, '', {}, io.BytesIO(json.dumps(invalid).encode()))):
                    with self.assertRaises(UncertainExecution):
                        runner(channel, b'{}')

    def test_rejected_output_is_never_retrievable_and_next_job_runs(self):
        for code in ['output_blocked', 'output_check_unavailable']:
            calls = []
            def runner(channel, body):
                calls.append(channel['job_id'])
                if len(calls) == 1:
                    raise OutputRejected(code)
                return PNG, {}
            queue = Dispatcher([{'id': 'slot-1', 'model': '2.1'}], runner)
            self.addCleanup(queue.stop)
            queue.start()
            job, _ = queue.submit('cuberouter-71', 'first-request', {'model': '2.1'})
            wait_for(lambda: queue.status('cuberouter-71', job['id'])['state'] == 'failed')
            self.assertEqual(queue.status('cuberouter-71', job['id'])['error'], code)
            self.assertEqual(queue.health()['quarantined_slots'], 0)
            self.assertEqual(queue.metrics()['content_bytes'], 0)
            with self.assertRaises(Rejected):
                queue.result('cuberouter-71', job['id'])
            duplicate, created = queue.submit('cuberouter-71', 'first-request', {'model': '2.1'})
            self.assertFalse(created)
            self.assertEqual(job['id'], duplicate['id'])
            next_job, _ = queue.submit('cuberouter-71', 'second-request', {'model': '2.1'})
            wait_for(lambda: queue.status('cuberouter-71', next_job['id'])['state'] == 'completed')
            self.assertEqual(queue.result('cuberouter-71', next_job['id']), PNG)
            self.assertEqual(calls, [job['id'], next_job['id']])
