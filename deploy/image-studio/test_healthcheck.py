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
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
import threading
import unittest
from unittest.mock import patch

from healthcheck import check_health


TOKEN = 'local-readiness-test-token-' + 'x' * 32


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.server.requests.append((self.path, self.headers.get('Authorization'),
                                     self.headers.get('X-Image-Owner')))
        if self.headers.get('Authorization') != 'Bearer ' + TOKEN:
            self.send_response(401)
            self.end_headers()
            return
        self.send_response(self.server.code)
        if self.server.redirect:
            self.send_header('Location', self.server.redirect)
        self.end_headers()
        self.wfile.write(self.server.body)

    def log_message(self, *args):
        pass


class HealthcheckTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = HTTPServer(('127.0.0.1', 0), Handler)
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join(timeout=2)

    def setUp(self):
        self.server.requests = []
        self.server.code = 200
        self.server.redirect = None
        self.server.body = json.dumps({'status': 'ready', 'healthy_slots': 8,
                                       'total_slots': 8, 'quarantined_slots': 0}).encode()
        self.env = {'IMAGE_STUDIO_DISPATCHER_TOKEN': TOKEN,
                    'IMAGE_STUDIO_PORT': str(self.server.server_port)}

    def test_authenticated_probe_does_not_require_an_owner_or_use_system_proxy(self):
        with patch.dict('os.environ', {'http_proxy': 'http://127.0.0.1:1'}):
            self.assertTrue(check_health(self.env))
        self.assertEqual(self.server.requests, [('/healthz', 'Bearer ' + TOKEN, None)])

    def test_degraded_capacity_remains_healthy_but_all_quarantined_fails(self):
        self.server.body = json.dumps({'status': 'degraded', 'healthy_slots': 1,
                                       'total_slots': 8, 'quarantined_slots': 7}).encode()
        self.assertTrue(check_health(self.env))
        self.server.body = json.dumps({'status': 'unavailable', 'healthy_slots': 0,
                                       'total_slots': 8, 'quarantined_slots': 8}).encode()
        self.server.code = 503
        self.assertFalse(check_health(self.env))

    def test_catchall_html_invalid_shape_or_capacity_cannot_pass(self):
        invalid = [b'<html>OK</html>', b'x' * 4097, b'null', b'[]',
                   {'status': 'ready'},
                   {'status': 'ready', 'healthy_slots': 0, 'total_slots': 8, 'quarantined_slots': 8},
                   {'status': 'ready', 'healthy_slots': True, 'total_slots': 8, 'quarantined_slots': 0},
                   {'status': 'ready', 'healthy_slots': 8, 'total_slots': 8, 'quarantined_slots': 1}]
        for body in invalid:
            with self.subTest(body=body):
                self.server.body = body if isinstance(body, bytes) else json.dumps(body).encode()
                self.assertFalse(check_health(self.env))

    def test_rejects_bad_credentials_and_does_not_follow_redirects(self):
        self.assertFalse(check_health(dict(self.env, IMAGE_STUDIO_DISPATCHER_TOKEN='z' * 48)))
        self.server.requests = []
        self.server.code = 302
        self.server.redirect = f'http://127.0.0.1:{self.server.server_port}/other'
        self.assertFalse(check_health(self.env))
        self.assertEqual(len(self.server.requests), 1)

    def test_invalid_local_configuration_makes_no_request(self):
        for override in ({'IMAGE_STUDIO_DISPATCHER_TOKEN': ''},
                         {'IMAGE_STUDIO_PORT': 'not-a-port'}, {'IMAGE_STUDIO_PORT': '65536'}):
            with self.subTest(override=override):
                self.assertFalse(check_health(dict(self.env, **override)))
        self.assertEqual(self.server.requests, [])


if __name__ == '__main__':
    unittest.main()
