#!/usr/bin/env python3
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

cover_packages = [
    "github.com/pulumi/pulumi/pkg/v3/...",
    "github.com/pulumi/pulumi/sdk/v3/...",
    "github.com/pulumi/pulumi/sdk/go/pulumi-language-go/v3/...",
    "github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3/...",
    "github.com/pulumi/pulumi/sdk/python/cmd/pulumi-language-python/v3/...",
]


dryrun = os.environ.get("PULUMI_TEST_DRYRUN", None) == "true"

def options(options_and_packages: List[str]):
    return [o for o in options_and_packages if not o.startswith('github.com/pulumi/pulumi')]


def packages(options_and_packages: List[str]):
    return [o for o in options_and_packages if o.startswith('github.com/pulumi/pulumi')]

root = pathlib.Path(__file__).absolute().parent.parent
integration_test_subset = os.environ.get('PULUMI_INTEGRATION_TESTS', None)
args = sys.argv[1:]

covdir = os.environ.get('PULUMI_TEST_COVERAGE_PATH', None)
covprofile = None
if covdir is not None:
    covprofile = f'{covdir}/go-test-{os.urandom(4).hex()}.cov'
elif '-cover' in args:
    wd = os.getcwd()
    covprofile = f'{wd}/go-test-{os.urandom(4).hex()}.cov'

if covprofile is not None:
    coverpkg = ','.join(cover_packages)
    args += [f'-coverprofile={covprofile}', f'-coverpkg={coverpkg}']

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
    print(heartbeat_str + " " + str(datetime.now()), file=sys.stderr) # Ensures GitHub receives stdout during long, silent package tests.
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
    junit_dir = root.joinpath('junit')
    if not junit_dir.is_dir():
        os.mkdir(str(junit_dir))
    json_file = str(test_results_dir.joinpath(f'{test_run}.json'))
    junit_file = str(junit_dir.joinpath(f'{test_run}-junit.xml'))
    args = ['gotestsum', '--junitfile', junit_file, '--jsonfile', json_file, '--packages', pkgs, '--'] + \
        opts
else:
    args = ['go', 'test'] + args

if not dryrun:
    try:
        print("Running: " + ' '.join(args))
        sp.check_call(args, shell=False)
        print("Completed: " + ' '.join(args))
    except sp.CalledProcessError as e:
        print("Failed: " + ' '.join(args))
        raise e
else:
    print("Would have run: " + ' '.join(args))

timer.cancel()
# rarely the python runtime will shut down, closing sys.stderr and crashing with a write to a closed
# sys.stderr. ensure that we exit with an OK status code, in case we race on the write in the timer:
sys.exit(0)
