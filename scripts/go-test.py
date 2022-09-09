"""
Wraps `go test`.
"""

from datetime import datetime
from typing import List
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

def options(options_and_packages: List[str]):
    return [o for o in options_and_packages if not o.startswith('github.com/pulumi/pulumi')]


def packages(options_and_packages: List[str]):
    return [o for o in options_and_packages if o.startswith('github.com/pulumi/pulumi')]


root = pathlib.Path(__file__).absolute().parent.parent
integration_test_subset = os.environ.get('PULUMI_INTEGRATION_TESTS', None)
args = sys.argv[1:]
cov = os.environ.get('PULUMI_TEST_COVERAGE_PATH', None)
if cov is not None:
    args = args + [f'-coverprofile={cov}/go-test-{os.urandom(4).hex()}.cov', '-coverpkg=github.com/pulumi/pulumi/pkg/v3/...,github.com/pulumi/pulumi/sdk/v3/...']

if integration_test_subset:
    print(f"Using test subset: {integration_test_subset}")
    args += ['-run', INTEGRATION_TESTS[integration_test_subset]]

if os.environ.get("CI") != "true":
    args += ['-v']


class RepeatTimer(threading.Timer):
    def run(self):
        while not self.finished.wait(self.interval):
            self.function(*self.args, **self.kwargs)

windows = platform.system() == 'Windows'

heartbeat_str = '💓' if not windows else 'heartbeat'

start_time = datetime.now()
def heartbeat():
    if not sys:
        # occurs during interpreter shutdown
        return
    print(heartbeat_str, file=sys.stderr) # Ensures GitHub receives stdout during long, silent package tests.
    sys.stdout.flush()
    sys.stderr.flush()

timer = RepeatTimer(10, heartbeat)
timer.daemon = True
timer.start()

if shutil.which('gotestsum') is not None:
    test_run = str(uuid.uuid4())

    opts = options(args)
    pkgs = " ".join(packages(args))

    test_results_dir = root.joinpath('test-results')
    if not test_results_dir.is_dir():
        os.mkdir(str(test_results_dir))

    json_file = str(test_results_dir.joinpath(f'{test_run}.json'))
    args = ['gotestsum', '--jsonfile', json_file, '--rerun-fails=2', '--packages', pkgs, '--'] + \
        opts

    print(' '.join(args))
    if not dryrun:
        sp.check_call(args, shell=False)
else:
    args = ['go', 'test'] + args
    print(' '.join(args))
    if not dryrun:
        sp.check_call(args, shell=False)

timer.cancel()
# ensure that we exit with a good status code, in case we race on the write in the timer. rarely the
# python runtime will shut down, closing sys.stderr and crashing with a write to a closed sys.stderr
exit(0)
