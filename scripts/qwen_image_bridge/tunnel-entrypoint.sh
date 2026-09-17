#!/bin/sh
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

set -eu
# A read-only Windows bind mount cannot guarantee OpenSSH key permissions.
# Keep the private copy in the container's tmpfs; never print its contents.
umask 077
mkdir -p /tmp/studio-ssh
for name in id_ed25519 config known_hosts; do
    cp "/run/ssh/$name" "/tmp/studio-ssh/$name"
    chmod 600 "/tmp/studio-ssh/$name"
done
exec ssh "$@"
