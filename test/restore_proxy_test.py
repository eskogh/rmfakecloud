"""Recovery orchestration checks; all network/device commands are replaced with mocks."""
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "scripts/restore-rmfakecloud-after-update.sh"
MOCK = r'''#!/usr/bin/env python3
import json, os, pathlib, sys, subprocess
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
with open(os.environ['MOCK_LOG'], 'a') as log:
    log.write(json.dumps([name, args]) + '\n')
if name == 'curl':
    pathlib.Path(args[args.index('-o')+1]).write_text('#!/bin/bash\n__ARCHIVE__\n')
elif name == 'ssh':
    cmd = args[-1]
    if '-M' in args or '-O' in args: sys.exit(0)
    if 'cat /sys/devices/soc0/machine' in cmd: print('reMarkable 2.0')
    elif cmd.startswith('wget '):
        print("wget: bad address 'rm.example.com'", file=sys.stderr)
        sys.exit(1)
    elif cmd.startswith('bash -s --'):
        body = sys.stdin.read()
        subprocess.run(['bash', '-n'], input=body, text=True, check=True)
        sys.exit(int(os.environ.get('VERIFY_FAIL', '0')))
    elif ' install' in cmd:
        if cmd.endswith(' install'):
            answers = sys.stdin.read()
            if answers != os.environ['TEST_CLOUD'] + '\n\n\n': sys.exit(3)
            sys.exit(int(os.environ.get('FALLBACK_FAIL', '0')))
        sys.exit(int(os.environ.get('INSTALL_FAIL', '0')))
'''

class RecoveryTest(unittest.TestCase):
    def run_recovery(self, cloud='https://rm.example.com', **flags):
        with tempfile.TemporaryDirectory(prefix='rm-recovery-test-') as directory:
            root = Path(directory)
            for name in ('ssh', 'scp', 'curl'):
                tool = root / name
                tool.write_text(MOCK)
                tool.chmod(0o755)
            log = root / 'calls.jsonl'
            env = {**os.environ, 'PATH': str(root) + os.pathsep + os.environ['PATH'],
                   'MOCK_LOG': str(log), 'TEST_CLOUD': cloud, **flags}
            result = subprocess.run(['bash', str(SCRIPT), '--cloud', cloud], env=env,
                                    capture_output=True, text=True)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            commands = [args[-1] for name, args in calls if name == 'ssh']
            return result, calls, commands

    def test_dns_failure_does_not_block_recovery(self):
        result, calls, commands = self.run_recovery()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(any(name == 'scp' for name, _ in calls))
        self.assertTrue(any(' setcloud ' in cmd for cmd in commands))
        self.assertIn('continuing USB recovery', result.stderr)

    def test_install_fallback_then_setcloud(self):
        result, _, commands = self.run_recovery(INSTALL_FAIL='1')
        self.assertEqual(result.returncode, 0, result.stderr)
        installs = [cmd for cmd in commands if ' install' in cmd]
        self.assertEqual(len(installs), 2)
        self.assertTrue(installs[1].endswith(' install'))
        self.assertTrue(any(' setcloud ' in cmd for cmd in commands))

    def test_failed_fallback_stops_without_success(self):
        result, _, commands = self.run_recovery(INSTALL_FAIL='1', FALLBACK_FAIL='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(' setcloud ' in cmd for cmd in commands))
        self.assertNotIn('restored successfully', result.stdout)

    def test_failed_verification_does_not_report_success(self):
        result, _, _ = self.run_recovery(VERIFY_FAIL='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('restored successfully', result.stdout)

    def test_cloud_url_remains_one_literal_argument(self):
        cloud = "https://rm.example.com/path?q=a'b&value=$(id)"
        result, _, commands = self.run_recovery(cloud=cloud)
        self.assertEqual(result.returncode, 0, result.stderr)
        for cmd in commands:
            if ' setcloud ' in cmd or ' install ' in cmd or cmd.startswith('wget '):
                self.assertEqual(shlex.split(cmd)[-1], cloud)

if __name__ == '__main__':
    unittest.main()
