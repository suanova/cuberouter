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

"""Create review-stack secrets locally without printing or overwriting them."""
import os
from pathlib import Path
import secrets
import sys

path = Path(sys.argv[1] if len(sys.argv) > 1 else ".env.media-studio")
try:
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
except FileExistsError:
    raise SystemExit("Configuration already exists; preserved without changes.")
with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
    handle.write("SESSION_SECRET=" + secrets.token_hex(32) + "\n")
    handle.write("MEDIA_STUDIO_BRIDGE_KEY=" + secrets.token_hex(32) + "\n")
    handle.write("QWEN_WORKFLOW_ORIGIN=http://127.0.0.1:18162\n")
    handle.write("STUDIO_BIND_ADDRESS=127.0.0.1\nSTUDIO_PORT=3001\n")
print("Created local configuration. No secrets printed. Follow the adapter README to connect the private worker.")
