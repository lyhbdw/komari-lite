#!/usr/bin/env python3
"""Isolated integration tests: real shell scripts + synthetic SQLite only.

Run: python3 -m unittest discover -s scripts/tests -v
No production service, credentials, downloads or database are accessed.
"""
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import sqlite3
import subprocess
import tempfile
import time
import unittest

REPO = Path(__file__).resolve().parents[2]
SCRATCH = Path('/root/.hermes/cache/scratch')

STUB = r'''#!/usr/bin/python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['KOMARI_TEST_ROOT']).resolve()
assert str(root).startswith('/root/.hermes/cache/scratch/')
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
with (root / 'events.jsonl').open('a') as f:
    f.write(json.dumps([name] + args) + '\n')
if name == 'docker':
    if 'inspect' in args:
        print('true' if '{{.State.Running}}' in args else 'image=fixture image_id=fixture')
    elif 'ps' in args:
        print('fixture-container')
    sys.exit(0)
if name == 'id':
    if args == ['-u']:
        print(os.environ.get('FAKE_UID', '1000'))
    elif '-u' in args:
        print('1000')
    sys.exit(0)
if name in ('chown', 'useradd', 'adduser', 'rc-update', 'uci'):
    sys.exit(0)
if name == 'ps':
    print('systemd')
    sys.exit(0)
if name == 'curl':
    import hashlib
    if args[-1].startswith('http://127.0.0.1:'):
        if os.environ.get('HEALTH_FAIL') == '1' and b'NEW-FIXTURE' in pathlib.Path(os.environ['TEST_BINARY']).read_bytes():
            sys.exit(7)
        print('{"status":"success"}')
        sys.exit(0)
    if '-I' in args or '-fsSLI' in args:
        print('content-length: ' + str((root / 'payload').stat().st_size))
        sys.exit(0)
    url = args[-1]
    output = args[args.index('-o') + 1] if '-o' in args else None
    payload = (root / 'payload').read_bytes()
    if os.environ.get('DOWNLOAD_FAIL') == '1':
        if output:
            pathlib.Path(output).write_bytes(b'partial')
        sys.exit(28)
    import hashlib
    if url.endswith('.sha256') and os.environ.get('CHECKSUM_FAIL') == '1':
        payload = b'0' * 64 + b'  fixture\n'
    elif url.endswith('.sha256'):
        payload = (hashlib.sha256(payload).hexdigest() + '  fixture\n').encode()
    if output:
        p = pathlib.Path(output).resolve()
        assert p.is_relative_to(root), p
        p.write_bytes(payload)
    else:
        sys.stdout.buffer.write(payload)
    sys.exit(0)
if name in ('systemctl', 'rc-service', 'initctl', 'fixture-procd'):
    state = root / 'running'
    if 'list-unit-files' in args:
        print('fixture-agent.service enabled')
        sys.exit(0)
    if 'is-enabled' in args or 'enabled' in args:
        sys.exit(0)
    if 'is-active' in args or 'status' in args or 'running' in args:
        if name == 'initctl':
            print('fixture-agent start/running, process 123' if state.exists() else 'fixture-agent stop/waiting')
            sys.exit(0)
        sys.exit(0 if state.exists() else 3)
    if 'stop' in args:
        state.unlink(missing_ok=True)
        sys.exit(0)
    if 'start' in args or '--now' in args:
        # Fail only the new binary: rollback of the old service can succeed.
        binary = pathlib.Path(os.environ['TEST_BINARY'])
        is_new = binary.exists() and b'NEW-FIXTURE' in binary.read_bytes()
        if is_new and os.environ.get('START_FAIL') == '1':
            sys.exit(1)
        if not (is_new and os.environ.get('STATUS_FAIL') == '1'):
            state.touch()
        sys.exit(0)
    sys.exit(0)
# These commands must never perform host package or service management.
sys.exit('forbidden fixture command: ' + name)
'''


class Fixture(unittest.TestCase):
    def setUp(self):
        SCRATCH.mkdir(parents=True, exist_ok=True)
        self.root = Path(tempfile.mkdtemp(prefix='komari-ops-', dir=SCRATCH))
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        for cmd in ('docker', 'systemctl', 'rc-service', 'rc-update', 'initctl',
                    'uci', 'id', 'chown', 'useradd', 'adduser', 'ps', 'curl',
                    'fixture-procd', 'apt', 'apt-get', 'yum', 'apk', 'opkg',
                    'service', 'killall', 'pkill', 'sudo'):
            p = self.bin / cmd
            p.write_text(STUB)
            p.chmod(0o755)
        self.env = {
            'PATH': f'{self.bin}:/usr/bin:/bin', 'HOME': str(self.root / 'home'),
            'TMPDIR': str(self.root), 'XDG_CONFIG_HOME': str(self.root / 'config'),
            'XDG_DATA_HOME': str(self.root / 'local'),
            'KOMARI_TEST_ROOT': str(self.root), 'KOMARI_ROOT': str(self.root),
            'KOMARI_DATA_DIR': str(self.root / 'data'),
            'KOMARI_BACKUP_DIR': str(self.root / 'backups'),
            'KOMARI_ROLLBACK_DIR': str(self.root / 'rollbacks'),
            'KOMARI_COMPOSE_FILE': str(self.root / 'compose.yml'),
            'KOMARI_CONTAINER_NAME': 'fixture-container', 'RETENTION_COUNT': '2',
            'KOMARI_SYSTEMD_DIR': str(self.root / 'systemd'),
            'KOMARI_INIT_DIR': str(self.root / 'init.d'),
            'KOMARI_UPSTART_DIR': str(self.root / 'upstart'),
            'KOMARI_RUN_DIR': str(self.root / 'run'),
            'KOMARI_RC_COMMON': str(self.root / 'rc.common'),
            'KOMARI_INIT_SYSTEM': 'systemd', 'KOMARI_SERVICE_USER': 'fixture-user',
            'KOMARI_INSTALL_DIR': str(self.root / 'server'),
            'KOMARI_SERVICE_NAME': 'fixture-server',
            'AGENT_TOKEN': 'synthetic-not-a-real-token',
            'AGENT_ENDPOINT': 'https://fixture.invalid',
            'TEST_BINARY': str(self.root / 'agent' / 'agent'),
            'LANG': 'C', 'LC_ALL': 'C',
        }
        for name in ('data', 'backups', 'home', 'rollbacks', 'agent', 'systemd',
                     'init.d', 'upstart', 'run', 'server'):
            (self.root / name).mkdir()
        (self.root / 'compose.yml').write_text('services: {}\n')
        (self.root / 'running').touch()
        for db in ('komari.db', 'metrics.db'):
            with sqlite3.connect(self.root / 'data' / db) as conn:
                conn.execute('create table fixture (value text)')
                conn.execute("insert into fixture values ('synthetic')")
        self.payload = self.root / 'payload'
        self.payload.write_text('#!/bin/sh\n# NEW-FIXTURE\nprintf "[\\\"smoke\\\"]\\n" >> "$KOMARI_TEST_ROOT/events.jsonl"\nexit 0\n')
        self.payload.chmod(0o755)

    def tearDown(self):
        shutil.rmtree(self.root)

    def events(self):
        path = self.root / 'events.jsonl'
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def run_script(self, script, *args, env=None):
        settings = self.env.copy()
        settings.update(env or {})
        return subprocess.run(['bash' if script == 'install-komari.sh' or script.startswith('scripts/') else 'sh',
                               str(REPO / script), *map(str, args)],
                              cwd=self.root, env=settings, text=True, capture_output=True, timeout=20)

    def backup(self):
        result = self.run_script('scripts/komari-backup.sh')
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return Path(result.stdout.splitlines()[-1])


class BackupRestoreTests(Fixture):
    def test_backup_cleanup_owns_only_its_namespace(self):
        keep = ['pre-restore-20000101T000000Z', '.restore-active', '.operator-private', 'manual-snapshot']
        for name in keep:
            (self.root / 'backups' / name).mkdir()
            (self.root / 'backups' / name / 'keep').write_text('synthetic')
        stale = self.root / 'backups' / '.backup-staging-dead'
        stale.mkdir()
        self.backup()
        for name in keep:
            self.assertTrue((self.root / 'backups' / name / 'keep').exists(), name)
        self.assertFalse(stale.exists())

    def test_two_backups_in_the_same_second_do_not_nest_staging(self):
        date = self.bin / 'date'
        date.write_text('#!/bin/sh\nprintf "20000101T000000Z\\n"\n')
        date.chmod(0o755)
        first = self.backup()
        second = self.run_script('scripts/komari-backup.sh')
        # Reject a collision rather than silently creating a hidden directory
        # inside the previous published backup.
        self.assertNotEqual(second.returncode, 0, second.stdout + second.stderr)
        self.assertFalse(any(p.name.startswith('.backup-staging-') for p in first.iterdir()))

    def test_restore_rejects_backup_lock_before_service_actions(self):
        backup = self.backup()
        lock = (self.root / 'backups' / '.lock').open('w')
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        before = len(self.events())
        try:
            result = self.run_script('scripts/komari-restore.sh', backup, '--yes')
        finally:
            lock.close()
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(any('stop' in event for event in self.events()[before:]))

    def test_restore_keeps_separate_rollbacks_and_database_permissions(self):
        backup = self.backup()
        result = self.run_script('scripts/komari-restore.sh', backup, '--yes')
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        rollbacks = list((self.root / 'rollbacks').glob('pre-restore-*'))
        self.assertEqual(len(rollbacks), 1)
        # A new ordinary backup cannot remove any restore rollback copy.
        date = self.bin / 'date'
        date.write_text('#!/bin/sh\nprintf "20990101T000000Z\\n"\n')
        date.chmod(0o755)
        self.backup()
        self.assertTrue((rollbacks[0] / 'komari.db').exists())
        self.assertEqual((self.root / 'data' / 'komari.db').stat().st_mode & 0o777, 0o600)

    def test_backup_cannot_run_while_restore_is_stopped(self):
        backup = self.backup()
        # Pause the fake container stop, while the actual restore holds its lock.
        docker = self.bin / 'docker'
        docker.write_text(STUB.replace("if name == 'docker':", "if name == 'docker':\n    if 'stop' in args:\n        (root / 'stopped').touch()\n        import time\n        deadline = time.monotonic() + 8\n        while not (root / 'release').exists() and time.monotonic() < deadline:\n            time.sleep(0.02)"))
        process = subprocess.Popen(['bash', str(REPO / 'scripts/komari-restore.sh'), str(backup), '--yes'],
                                   cwd=self.root, env=self.env, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            deadline = time.monotonic() + 5
            while not (self.root / 'stopped').exists() and process.poll() is None and time.monotonic() < deadline:
                time.sleep(0.02)
            self.assertTrue((self.root / 'stopped').exists())
            result = self.run_script('scripts/komari-backup.sh')
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        finally:
            (self.root / 'release').touch()
            stdout, stderr = process.communicate(timeout=10)
        self.assertEqual(process.returncode, 0, stdout + stderr)


    def test_restore_rollback_retention_only_prunes_owned_rollbacks(self):
        backup = self.backup()
        keep = self.root / 'rollbacks' / 'manual-recovery'
        keep.mkdir()
        for stamp in ('20000101', '20000102'):
            (self.root / 'rollbacks' / f'pre-restore-{stamp}T000000Z.aaaaaa').mkdir()
        for _ in range(2):
            result = self.run_script('scripts/komari-restore.sh', backup, '--yes',
                                     env={'ROLLBACK_RETENTION_COUNT': '1'})
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(len(list((self.root / 'rollbacks').glob('pre-restore-*'))), 1)
            self.assertTrue(keep.exists())

    def test_restore_retention_always_keeps_newest_created_rollback(self):
        backup = self.backup()
        date = self.bin / 'date'
        date.write_text('#!/bin/sh\nprintf "20000101T000000Z\\n"\n')
        date.chmod(0o755)
        # Sort order of a random mktemp suffix is not creation order.
        previous = self.root / 'rollbacks' / 'pre-restore-20000101T000000Z.zzzzzz'
        previous.mkdir()
        result = self.run_script('scripts/komari-restore.sh', backup, '--yes',
                                 env={'ROLLBACK_RETENTION_COUNT': '1'})
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        remaining = list((self.root / 'rollbacks').glob('pre-restore-*'))
        self.assertEqual(len(remaining), 1)
        self.assertTrue((remaining[0] / 'komari.db').exists())

    def test_compressed_backup_restores_and_removes_only_its_unpacking_dir(self):
        backup = self.backup()
        archive = self.root / 'backups' / 'fixture.tar.zst'
        subprocess.run(['tar', '-C', str(backup), '--zstd', '-cf', str(archive), '.'],
                       cwd=self.root, env=self.env, check=True)
        foreign = self.root / 'backups' / '.restore-foreign'
        foreign.mkdir()
        result = self.run_script('scripts/komari-restore.sh', archive, '--yes')
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(list((self.root / 'backups').glob('.restore-*')), [foreign])

    def test_restore_bad_checksum_cannot_stop_container(self):
        backup = self.backup()
        (backup / 'metrics.db').write_bytes(b'invalid')
        result = self.run_script('scripts/komari-restore.sh', backup, '--yes')
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(any('stop' in event for event in self.events()))


class AgentTests(Fixture):
    def agent(self, env=None):
        digest = hashlib.sha256(self.payload.read_bytes()).hexdigest()
        return self.run_script('agent/install.sh', '--install-dir', self.root / 'agent',
                               '--install-service-name', 'fixture-agent', '--local-binary',
                               self.payload, '--sha256', digest, env=env)

    def test_procd_runtime_executes_the_generated_runner(self):
        # Kernel dispatches /bin/sh rc.common <service> <action>; both files
        # are synthetic and entirely inside scratch, not the host init tree.
        common = self.root / 'rc.common'
        common.write_text('''#!/bin/sh
service_script=$1
shift
procd_open_instance() { :; }
procd_close_instance() { :; }
procd_set_param() {
    if [ "$1" = command ]; then
        [ "$2" = "$KOMARI_TEST_ROOT/agent/run-agent.sh" ] || exit 42
        [ -x "$2" ] || exit 43
        printf '["procd-command","%s"]\\n' "$2" >> "$KOMARI_TEST_ROOT/events.jsonl"
    fi
}
. "$service_script"
if [ "$1" = start ]; then start_service; fi
exec "$KOMARI_TEST_ROOT/bin/fixture-procd" "$@"
''')
        result = self.agent(env={'FAKE_UID': '0', 'KOMARI_INIT_SYSTEM': 'procd'})
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn(['procd-command', str(self.root / 'agent' / 'run-agent.sh')], self.events())
        self.assertFalse(any(event[0] == 'rc-service' for event in self.events()))

    def test_supervisor_status_failure_restores_old_installation(self):
        old = self.root / 'agent' / 'agent'
        old.write_text('#!/bin/sh\n# OLD-FIXTURE\nexit 0\n')
        old.chmod(0o755)
        result = self.agent(env={'STATUS_FAIL': '1'})
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn('OLD-FIXTURE', old.read_text())
        self.assertTrue((self.root / 'running').exists())

    def test_unexecutable_download_does_not_replace_or_stop_old(self):
        old = self.root / 'agent' / 'agent'
        old.write_text('old-binary')
        old.chmod(0o755)
        self.payload.write_text('invalid executable image\n')
        result = self.agent()
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(old.read_text(), 'old-binary')
        self.assertFalse(any('stop' in event for event in self.events()))

    def test_user_service_start_failure_restores_old_installation(self):
        old = self.root / 'agent' / 'agent'
        old.write_text('#!/bin/sh\n# OLD-FIXTURE\nexit 0\n')
        old.chmod(0o755)
        unit = self.root / 'config' / 'systemd' / 'user' / 'fixture-agent.service'
        unit.parent.mkdir(parents=True)
        unit.write_text('old-unit\n')
        credential = self.root / 'agent' / '.agent.env'
        credential.write_text('synthetic-old-credential\n')
        credential.chmod(0o600)
        old_bytes = old.read_bytes()
        result = self.agent(env={'START_FAIL': '1'})
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(old.read_bytes(), old_bytes)
        self.assertEqual(unit.read_text(), 'old-unit\n')
        self.assertEqual(credential.read_text(), 'synthetic-old-credential\n')
        self.assertTrue((self.root / 'running').exists())
        events = self.events()
        smoke = next(i for i, event in enumerate(events) if event == ['smoke'])
        stop = next(i for i, event in enumerate(events) if 'stop' in event)
        self.assertLess(smoke, stop)


    def test_all_other_supervisors_restore_old_service_on_failure(self):
        for init in ('systemd', 'openrc', 'upstart'):
            with self.subTest(init=init):
                old = self.root / 'agent' / 'agent'
                old.write_text('#!/bin/sh\n# OLD-FIXTURE\nexit 0\n')
                old.chmod(0o755)
                if init == 'systemd':
                    service = self.root / 'systemd' / 'fixture-agent.service'
                elif init == 'openrc':
                    service = self.root / 'init.d' / 'fixture-agent'
                else:
                    service = self.root / 'upstart' / 'fixture-agent.conf'
                service.write_text('old-unit\n')
                (self.root / 'running').touch()
                result = self.agent(env={'FAKE_UID': '0', 'KOMARI_INIT_SYSTEM': init, 'START_FAIL': '1'})
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn('OLD-FIXTURE', old.read_text())
                self.assertEqual(service.read_text(), 'old-unit\n')
                self.assertTrue((self.root / 'running').exists())

    def test_agent_network_attempts_all_keep_tls_and_time_bounds(self):
        result = self.run_script('agent/install.sh', '--install-dir', self.root / 'agent',
                                 '--install-service-name', 'fixture-agent', env={'DOWNLOAD_FAIL': '1'})
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        downloads = [event for event in self.events() if event[0] == 'curl']
        self.assertGreaterEqual(len(downloads), 3)
        for event in downloads:
            self.assertIn('--max-time', event)
            self.assertIn('--proto', event)
            self.assertIn('--tlsv1.2', event)

    def test_agent_checksum_mismatch_never_replaces_or_stops_old(self):
        old = self.root / 'agent' / 'agent'
        old.write_text('OLD-FIXTURE')
        result = self.run_script('agent/install.sh', '--install-dir', self.root / 'agent',
                                 '--install-service-name', 'fixture-agent', env={'CHECKSUM_FAIL': '1'})
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(old.read_text(), 'OLD-FIXTURE')
        self.assertFalse(any('stop' in event for event in self.events()))


class ServerTests(Fixture):
    def server(self, operation, env=None):
        old = self.root / 'server' / 'komari'
        unit = self.root / 'systemd' / 'fixture-server.service'
        if old.exists() and not unit.exists():
            unit.write_text(f'ExecStart={old} server -l 127.0.0.1:25774\n')
        # Evaluate the actual script's definitions, suppressing only its menu
        # entrypoint. Every operational function remains unchanged.
        source = (REPO / 'install-komari.sh').read_text().split('# Main execution')[0]
        harness = self.root / 'server-harness.sh'
        harness.write_text(source + '\n' + '''
INSTALL_DIR="$KOMARI_INSTALL_DIR"
DATA_DIR="$KOMARI_ROOT"
SERVICE_NAME="$KOMARI_SERVICE_NAME"
BINARY_PATH="$INSTALL_DIR/komari"
BACKUP_DIR="$INSTALL_DIR/backup"
DATA_BACKUP_DIR="$DATA_DIR/data/backup"
ui_msgbox() { printf '%s\\n' "$*"; }
select_edition() { :; }
select_channel() { :; }
ui_input() { printf '25774\\n'; }
show_access_info() { :; }
''' + operation + '\n')
        settings = self.env.copy()
        settings.update({'TEST_BINARY': str(self.root / 'server' / 'komari')})
        settings.update(env or {})
        return subprocess.run(['bash', str(harness)], cwd=self.root, env=settings,
                              text=True, capture_output=True, timeout=20)

    def test_upgrade_failed_download_never_stops_or_changes_old_binary(self):
        old = self.root / 'server' / 'komari'
        old.write_text('OLD-FIXTURE')
        result = self.server('upgrade_komari', {'DOWNLOAD_FAIL': '1'})
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(old.read_text(), 'OLD-FIXTURE')
        self.assertFalse(any('stop' in event for event in self.events()))

    def test_upgrade_start_failure_returns_error_and_restores_old_binary(self):
        old = self.root / 'server' / 'komari'
        old.write_text('#!/bin/sh\n# OLD-FIXTURE\nexit 0\n')
        old.chmod(0o755)
        old_bytes = old.read_bytes()
        result = self.server('upgrade_komari', {'START_FAIL': '1'})
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(old.read_bytes(), old_bytes)
        self.assertTrue((self.root / 'running').exists())
        self.assertTrue(any('stop' in event for event in self.events()))


    def test_first_install_failure_leaves_no_final_binary(self):
        for failure in ('DOWNLOAD_FAIL', 'START_FAIL'):
            with self.subTest(failure=failure):
                result = self.server('install_binary', {failure: '1'})
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertFalse((self.root / 'server' / 'komari').exists())
                self.assertFalse((self.root / 'systemd' / 'fixture-server.service').exists())
                self.assertFalse(list((self.root / 'server').glob('.komari-download.*')))

    def test_upgrade_preflight_failure_never_stops_service(self):
        old = self.root / 'server' / 'komari'
        old.write_text('OLD-FIXTURE')
        self.payload.write_bytes(b'not an executable\n')
        result = self.server('upgrade_komari')
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(old.read_text(), 'OLD-FIXTURE')
        self.assertFalse(any('stop' in event for event in self.events()))

    def test_upgrade_success_uses_same_filesystem_staging_before_stop(self):
        old = self.root / 'server' / 'komari'
        unit = self.root / 'systemd' / 'fixture-server.service'
        unit.write_text(f'ExecStart={old} server -l 127.0.0.1:25774\n')
        old = self.root / 'server' / 'komari'
        old.write_text('OLD-FIXTURE')
        result = self.server('upgrade_komari')
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        events = self.events()
        smoke = next(i for i, event in enumerate(events) if event == ['smoke'])
        stop = next(i for i, event in enumerate(events) if 'stop' in event)
        self.assertLess(smoke, stop)
        self.assertEqual(old.read_bytes(), self.payload.read_bytes())
        for event in events:
            if event[0] == 'curl':
                self.assertIn('--max-time', event)
                if event[-1].startswith('http://127.0.0.1:'):
                    continue
                self.assertIn('--proto', event)
                self.assertIn('--proto-redir', event)
                if '-o' in event:
                    path = Path(event[event.index('-o') + 1])
                    self.assertEqual(path.parent, old.parent)
                    self.assertNotEqual(path, old)


    def test_upgrade_http_health_failure_rolls_back_old_binary(self):
        old = self.root / 'server' / 'komari'
        old.write_text('#!/bin/sh\n# OLD-FIXTURE\nexit 0\n')
        old.chmod(0o755)
        unit = self.root / 'systemd' / 'fixture-server.service'
        unit.write_text(f'ExecStart={old} server -l 127.0.0.1:25774\n')
        result = self.server('upgrade_komari', {'HEALTH_FAIL': '1'})
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn('OLD-FIXTURE', old.read_text())
        self.assertTrue((self.root / 'running').exists())


if __name__ == '__main__':
    unittest.main()
