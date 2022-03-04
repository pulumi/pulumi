"""
Wraps `go test`.
"""

from datetime import datetime
from integration_test_subsets import INTEGRATION_TESTS
import os
import pathlib
import platform
import shutil
import subprocess as sp
import sys
import uuid
import threading

dryrun = os.environ.get("PULUMI_TEST_DRYRUN", None) == "true"

root = pathlib.Path(__file__).absolute().parent.parent
integration_test_subset = os.environ.get('PULUMI_INTEGRATION_TESTS', None)
options = sys.argv[1:]
cov = os.environ.get('PULUMI_TEST_COVERAGE_PATH', None)
if cov is not None:
    options = options + [f'-coverprofile={cov}/go-test-{os.urandom(4).hex()}.cov', '-coverpkg=github.com/pulumi/pulumi/pkg/v3/...,github.com/pulumi/pulumi/sdk/v3/...']

if integration_test_subset:
    print(f"Using test subset: {integration_test_subset}")
    options += ['-run', INTEGRATION_TESTS[integration_test_subset]]

if os.environ.get("CI") != "true":
    options += ['-v']


class RepeatTimer(threading.Timer):
    def run(self):
        while not self.finished.wait(self.interval):
            self.function(*self.args, **self.kwargs)

windows = platform.system() == 'Windows'

heartbeat_str = '💓' if not windows else 'heartbeat'

start_time = datetime.now()
def heartbeat():
    print(heartbeat_str, file=sys.stderr) # Ensures GitHub receives stdout during long, silent package tests.
    sys.stdout.flush()
    sys.stderr.flush()

timer = RepeatTimer(10, heartbeat)
timer.daemon = True
timer.start()

if shutil.which('gotestsum') is not None:
    test_run = str(uuid.uuid4())

    test_results_dir = root.joinpath('test-results')
    if not test_results_dir.is_dir():
        os.mkdir(str(test_results_dir))

    json_file = str(test_results_dir.joinpath(f'{test_run}.json'))
    junit_file = str(test_results_dir.joinpath(f'{test_run}.xml'))
    args = ['gotestsum', '--jsonfile', json_file, '--junitfile', junit_file, '--'] + \
        options

    print(' '.join(args))
    if not dryrun:
        sp.check_call(args, shell=False)
else:
    args = ['go', 'test'] + options
    print(' '.join(args))
    if not dryrun:
        sp.check_call(args, shell=False)
