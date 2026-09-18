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

"""Read-only adapter/worker diagnosis; no inference, account data or secret output."""
import argparse
import json
import os
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, ProxyHandler, HTTPRedirectHandler, build_opener


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--web-origin', help='Also verify the embedded studio page and template asset')
parser.add_argument('--web-only', action='store_true', help='Check a UI-only build without the adapter')
args = parser.parse_args()
http = build_opener(ProxyHandler({}), NoRedirect())
if args.web_only and not args.web_origin:
    parser.error('--web-only requires --web-origin')
if args.web_origin:
    for path, expected in [('/media-studio', 'text/html'), ('/studio-templates/pet-comic-v1.jpg', 'image/jpeg')]:
        try:
            with http.open(Request(args.web_origin.rstrip('/') + path, method='HEAD'), timeout=10) as response:
                if response.status != 200 or response.headers.get_content_type() != expected:
                    raise SystemExit('Unexpected page or asset response: ' + path)
        except (HTTPError, URLError, TimeoutError):
            raise SystemExit('Page or asset unavailable or redirected: ' + path)
    print('Embedded studio page and template asset return HTTP 200 without redirects.')
if args.web_only:
    sys.exit(0)

port = os.environ.get("QWEN_BRIDGE_PORT", "18163")
key = os.environ.get("QWEN_BRIDGE_API_KEY", "")
if len(key) < 24:
    raise SystemExit("Adapter key is missing or too short.")
request = Request("http://127.0.0.1:" + port + "/studio/1/config",
                  headers={"Authorization": "Bearer " + key})
try:
    with http.open(request, timeout=25) as response:
        config = json.load(response)
except HTTPError as error:
    raise SystemExit("Adapter configuration request failed: HTTP " + str(error.code))
except (URLError, TimeoutError, ValueError):
    raise SystemExit("Adapter is unreachable or returned invalid configuration.")
health = config.get("health", {})
print(json.dumps({"models": config.get("models"), "health": health}, ensure_ascii=False))
if any(health.get(name) not in {"ready", "busy"} for name in ("create", "edit", "tools")):
    raise SystemExit("Worker connection is not ready; check the SSH tunnel and private workflow origin.")
print("Adapter and existing worker are reachable. Configure both CubeRouter channels before generating.")
