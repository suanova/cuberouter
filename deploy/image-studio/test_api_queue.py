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
"""Shared browser/API admission, private envelope and durable job identity."""
import json
from unittest.mock import patch
from urllib.request import Request
from dispatcher import channel_runner
import test_dispatcher
from test_dispatcher import payload
from test_queue import PNG, wait_for


class APIQueueTests(test_dispatcher.HTTPTests):
    def setUp(self):
        super().setUp()
        self.server.api_enabled = True

    def api_payload(self):
        p = payload()
        p['_api'] = dict(token_id=11, group='image-studio', client_ip='127.0.0.1', request_id='api-request-000001')
        return p

    def test_web_cannot_inject_api_identity(self):
        self.assertEqual(self.request('/jobs', self.api_payload())[0], 400)
        self.assertEqual(self.calls, [])

    def test_shared_owner_limit_fair_queue_and_stable_id(self):
        key = 'a' * 64
        status, job = self.request('/api-jobs', self.api_payload(), key=key)
        self.assertEqual(status, 202)
        self.assertEqual(job['id'], key[:32])
        wait_for(lambda: len(self.calls) == 1)
        # Same account cannot evade the existing Studio limit by using an API key.
        self.assertEqual(self.request('/jobs', payload(), key='web-request-000001')[0], 429)
        _, web = self.request('/jobs', payload(), owner='cuberouter-2')
        self.assertEqual(web['state'], 'queued')
        status, duplicate = self.request('/api-jobs', self.api_payload(), key=key)
        self.assertEqual(status, 200)
        self.assertEqual(duplicate['id'], job['id'])
        self.gate.set()
        wait_for(lambda: self.dispatch.status('cuberouter-2', web['id'])['state'] == 'completed')
        self.assertEqual(len(self.calls), 2)

    def test_direct_worker_mode_does_not_advertise_or_accept_api(self):
        self.server.api_enabled = False
        self.assertIsNone(self.request('/session')[1]['api_protocol'])
        self.assertEqual(self.request('/api-jobs', self.api_payload(), key='b' * 64)[0], 503)

    def test_runner_uses_separate_callback_without_forwarding_key(self):
        class Response:
            headers = {'X-Image-Studio-Channel': '7'}
            def __enter__(self): return self
            def __exit__(self, *args): pass
            def read(self, *args):
                import base64
                return json.dumps({'data': [{'b64_json': base64.b64encode(PNG).decode()}]}).encode()
        with patch('dispatcher.HTTP.open', return_value=Response()) as send:
            channel_runner('http://127.0.0.1:9999', 'r' * 40)(
                {'owner': 'cuberouter-1', 'job_id': 'a' * 32}, json.dumps(self.api_payload()).encode())
            request = send.call_args.args[0]
            self.assertEqual(request.full_url, 'http://127.0.0.1:9999/internal/image-studio/execute-api')
            self.assertEqual(json.loads(request.data)['_api']['token_id'], 11)
            self.assertEqual(request.get_header('Authorization'), 'Bearer ' + 'r' * 40)
