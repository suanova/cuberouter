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
"""Check authenticated queue readiness without putting secrets in process arguments."""
import json
import os
import sys
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def check_health(environ=None):
    environ = os.environ if environ is None else environ
    token = environ.get('IMAGE_STUDIO_DISPATCHER_TOKEN', '')
    try:
        port = int(environ.get('IMAGE_STUDIO_PORT', '18195'))
        if len(token) < 32 or not 1 <= port <= 65535:
            return False
        request = Request(f'http://127.0.0.1:{port}/healthz',
                          headers={'Authorization': 'Bearer ' + token})
        # Readiness is local to this container. Never send the bearer through a
        # configured HTTP proxy or follow redirects to another destination.
        with build_opener(ProxyHandler({}), NoRedirect()).open(request, timeout=3) as response:
            body = response.read(4097)
            if response.status != 200 or len(body) > 4096:
                return False
        data = json.loads(body)
        if not isinstance(data, dict) or data.get('status') not in ('ready', 'degraded'):
            return False
        healthy, total, quarantined = (data.get(key) for key in (
            'healthy_slots', 'total_slots', 'quarantined_slots'))
        if any(type(value) is not int for value in (healthy, total, quarantined)):
            return False
        return 0 < healthy <= total and 0 <= quarantined <= total - healthy
    except (OSError, ValueError):
        return False


if __name__ == '__main__':
    if not check_health():
        print('Image Studio queue unavailable', file=sys.stderr)
        raise SystemExit(1)
