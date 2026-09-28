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

"""Render an opt-in Compose overlay; never deploy, register channels or rotate admin keys."""
import argparse
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import subprocess
import sys

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
DISPATCHER = 'image-studio-dispatcher'
NETWORK = 'image-studio-private'


def channel_ids(value):
    parts = [part.strip() for part in value.split(',')]
    if not 1 <= len(parts) <= 8 or any(not re.fullmatch(r'[1-9][0-9]*', part) for part in parts):
        raise ValueError('Supply 1-8 positive channel IDs from the TARGET CubeRouter database.')
    if len(set(parts)) != len(parts):
        raise ValueError('Channel IDs must be unique, one per physical GPU.')
    return parts


def make_overlay(base, service, ids, tokens, port, image=None):
    if service not in base.get('services', {}):
        raise ValueError('Gateway service is absent from the existing Compose project.')
    gateway = base['services'][service]
    if gateway.get('network_mode'):
        raise ValueError('Use a gateway with Compose networking; network_mode requires a manual deployment.')
    if gateway.get('scale', 1) != 1 or gateway.get('deploy', {}).get('replicas', 1) != 1:
        raise ValueError('This GPU reservation pool requires ONE CubeRouter replica.')
    if DISPATCHER in base['services'] or NETWORK in base.get('networks', {}):
        raise ValueError('Reserved Image Studio service/network already exists; inspect it before modifying.')
    if not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9_-]*', service) or not 1 <= port <= 65535:
        raise ValueError('Invalid gateway service name or port.')
    dispatcher_token, relay_token = tokens['dispatcher_token'], tokens['relay_token']
    if min(len(dispatcher_token), len(relay_token)) < 32 or dispatcher_token == relay_token:
        raise ValueError('Two different internal secrets of at least 32 characters are required.')
    # Preserve an implicit default network when the overlay adds an explicit one.
    networks = dict(gateway.get('networks') or {'default': None})
    networks[NETWORK] = None
    worker = {
        'restart': 'unless-stopped', 'read_only': True, 'cap_drop': ['ALL'],
        'security_opt': ['no-new-privileges:true'], 'mem_limit': '1g',
        'networks': {NETWORK: None},
        'environment': {
            'IMAGE_STUDIO_BIND': '0.0.0.0', 'IMAGE_STUDIO_PORT': '18195',
            'IMAGE_STUDIO_DISPATCHER_TOKEN': dispatcher_token,
            'IMAGE_STUDIO_RELAY_URL': f'http://{service}:{port}',
            'IMAGE_STUDIO_RELAY_TOKEN': relay_token,
            'IMAGE_STUDIO_RELAY_CONCURRENCY': str(len(ids)),
        },
        'healthcheck': {
            'test': ['CMD', 'python', '-c',
                     "import socket; socket.create_connection(('127.0.0.1',18195),2).close()"],
            'interval': '30s', 'timeout': '5s', 'retries': 3,
        },
    }
    if image:
        worker['image'] = image
    else:
        worker['build'] = {'context': str(HERE)}
    return {'services': {
        service: {'networks': networks, 'environment': {
            'IMAGE_STUDIO_DISPATCHER_URL': f'http://{DISPATCHER}:18195',
            'IMAGE_STUDIO_DISPATCHER_TOKEN': dispatcher_token,
            'IMAGE_STUDIO_RELAY_TOKEN': relay_token,
            'IMAGE_STUDIO_CHANNEL_MODEL': 'qwen-image-2.1',
            'IMAGE_STUDIO_CHANNEL_IDS': ','.join(ids),
            'IMAGE_STUDIO_AUDIT_ENABLED': 'true',
            'IMAGE_STUDIO_ALLOWED_GROUPS': 'image-studio',
        }},
        DISPATCHER: worker,
    }, 'networks': {NETWORK: {'internal': True}}}


def private_write(path, value):
    if path.is_symlink():
        raise ValueError('Refusing a symlink output file.')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8', newline='\n') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')
    os.chmod(path, 0o600)


def load_tokens(directory, project):
    directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    path = directory / 'internal-secrets.json'
    if path.is_symlink():
        raise ValueError('Refusing a symlink secret file.')
    if path.exists():
        tokens = json.loads(path.read_text(encoding='utf-8'))
        if tokens.get('project') != project:
            raise ValueError('This secret directory belongs to another Compose project.')
        return tokens
    tokens = {'project': project, 'dispatcher_token': secrets.token_urlsafe(36),
              'relay_token': secrets.token_urlsafe(36)}
    private_write(path, tokens)
    return tokens


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--compose', action='append', required=True, help='Existing base Compose file(s), in original order')
    parser.add_argument('--env-file', action='append', default=[], help='Existing Compose env file(s), if used by the deployment')
    parser.add_argument('--project-name', required=True, help='EXISTING Docker Compose project name, not a new one')
    parser.add_argument('--gateway-service', default='cuberouter')
    parser.add_argument('--gateway-port', type=int, default=3000)
    parser.add_argument('--channel-ids', required=True)
    parser.add_argument('--output', required=True, help='Private directory OUTSIDE the checkout, reused on subsequent runs')
    parser.add_argument('--dispatcher-image', help='Matching released dispatcher image; omit to build the checked-out source')
    args = parser.parse_args(argv)
    ids = channel_ids(args.channel_ids)
    output = Path(args.output).expanduser().resolve()
    if output.is_relative_to(REPO):
        raise ValueError('Secret output must be outside the Git checkout.')
    compose_files = [Path(p).expanduser().resolve(strict=True) for p in args.compose]
    command = ['docker', 'compose', '--project-directory', str(compose_files[0].parent),
               '--project-name', args.project_name]
    for path in args.env_file:
        command += ['--env-file', str(Path(path).expanduser().resolve(strict=True))]
    for path in compose_files:
        command += ['-f', str(path)]
    resolved = subprocess.run(command + ['config', '--format', 'json'], capture_output=True, text=True, timeout=45)
    if resolved.returncode:
        raise ValueError('Base Compose configuration failed validation. Check it separately; no secrets were printed.')
    base = json.loads(resolved.stdout)
    tokens = load_tokens(output, args.project_name)
    overlay = make_overlay(base, args.gateway_service, ids, tokens, args.gateway_port, args.dispatcher_image)
    destination = output / 'compose.image-studio.json'
    private_write(destination, overlay)
    command += ['-f', str(destination)]
    verified = subprocess.run(command + ['config', '--quiet'], capture_output=True, text=True, timeout=45)
    if verified.returncode:
        raise ValueError('Generated Compose overlay failed validation; do not deploy it.')
    print(f'Validated overlay: {destination}')
    print(f'GPU channels: {len(ids)}; one gateway replica; audit enabled; dispatcher has no public ports.')
    print('No service or channel was changed. After confirming worker connectivity and the gateway image version:')
    print(shlex.join(command + ['up', '-d', '--build', '--no-deps', DISPATCHER, args.gateway_service]))
    print('Use this same full Compose command for later updates. Do not run it with a new project name.')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.SubprocessError) as exc:
        print(f'Configuration not ready: {exc}', file=sys.stderr)
        raise SystemExit(1)
