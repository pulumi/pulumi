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
retries = int(os.environ.get("PULUMI_TEST_RETRIES", "0"))

def options(options_and_packages: List[str]):
    return [o for o in options_and_packages if not o.startswith('github.com/pulumi/pulumi')]


def packages(options_and_packages: List[str]):
    return [o for o in options_and_packages if o.startswith('github.com/pulumi/pulumi')]

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

def get_command(args: List[str]):
    root = pathlib.Path(__file__).absolute().parent.parent
    integration_test_subset = os.environ.get('PULUMI_INTEGRATION_TESTS', None)
    cov = os.environ.get('PULUMI_TEST_COVERAGE_PATH', None)
    if cov is not None:
        args = args + [f'-coverprofile={cov}/go-test-{os.urandom(4).hex()}.cov', '-coverpkg=github.com/pulumi/pulumi/pkg/v3/...,github.com/pulumi/pulumi/sdk/v3/...']

    if integration_test_subset:
        print(f"Using test subset: {integration_test_subset}")
        args += ['-run', INTEGRATION_TESTS[integration_test_subset]]

    if os.environ.get("CI") != "true":
        args += ['-v']

    if shutil.which('gotestsum') is not None:
        test_run = str(uuid.uuid4())

        opts = options(args)
        pkgs = " ".join(packages(args))

        test_results_dir = root.joinpath('test-results')
        if not test_results_dir.is_dir():
            os.mkdir(str(test_results_dir))

        json_file = str(test_results_dir.joinpath(f'{test_run}.json'))
        args = ['gotestsum', '--jsonfile', json_file, '--rerun-fails=1', '--packages', pkgs, '--'] + \
            opts
    else:
        args = ['go', 'test'] + args

    return args


def run_tests():
    test_parallelism = int(os.environ.get("TESTPARALLELISM", "8"))
    pkg_parallelism = int(os.environ.get("PKGPARALLELISM", "2"))
    shuffle = os.environ.get("TEST_SHUFFLE", "off") # TODO: default to on

    attempts = 0
    for attempt in range(retries + 1):
        attempts = attempt + 1

        args = sys.argv[1:]
        args = ['-parallel', str(test_parallelism), '-p', str(pkg_parallelism), "-shuffle", shuffle] + args
        args = get_command(args)
        try:
            if not dryrun:
                sp.check_call(args, shell=False)
            else:
                print("Would have run: " + ' '.join(args))

            return (True, attempts)
        except:
            print(f"::error Failed to run tests. Attempt {attempt + 1}/{retries + 1}")
            test_parallelism = max(1, test_parallelism // 2)
            pkg_parallelism = max(1, pkg_parallelism // 2)
            shuffle = "off"

    return (False, attempts)

def main():
    success, attempts = run_tests()

    success = str(success).lower()
    retried = str(attempts > 1).lower()
    print(f"::set-output name=TEST_SUCCESS::{success}")
    print(f"::set-output name=TEST_RETRIED::{retried}")

    if not success:
        if os.environ.get("CI", "") == "true":
            print("::error::Tests failed")
        else:
            sys.exit(1)

main()

timer.cancel()
# ensure that we exit with a good status code, in case we race on the write in the timer. rarely the
# python runtime will shut down, closing sys.stderr and crashing with a write to a closed sys.stderr
sys.exit(0)
