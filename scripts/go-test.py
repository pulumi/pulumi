"""
Wraps `go test`.
"""

from integration_test_subsets import INTEGRATION_TESTS
import os
import pathlib
import shutil
import subprocess as sp
import sys
import uuid

dryrun = os.environ.get("PULUMI_TEST_DRYRUN", None) == "true"

def options(options_and_packages):
    return [o for o in options_and_packages if '/' not in o]


def packages(options_and_packages):
    return [o for o in options_and_packages if '/' in o]


root = pathlib.Path(__file__).absolute().parent.parent
integration_test_subset = os.environ.get('PULUMI_INTEGRATION_TESTS', None)
options_and_packages = sys.argv[1:]
packages = packages(options_and_packages)

if not packages:
    print(f'No packages matching PULUMI_TEST_SUBSET={integration_test_subset}')
    sys.exit(0)

options = options(options_and_packages)
cov = os.environ.get('PULUMI_TEST_COVERAGE_PATH', None)
if cov is not None:
    options = options + [f'-coverprofile={cov}/go-test-{os.urandom(4).hex()}.cov', '-coverpkg=github.com/pulumi/pulumi/pkg/v3/...,github.com/pulumi/pulumi/sdk/v3/...']

if integration_test_subset:
    print(f"Using test subset: {integration_test_subset}")
    options += ['-run', INTEGRATION_TESTS[integration_test_subset]]

if os.environ.get("CI") != "true":
    options += ['-v']

options_and_packages = options + packages


if shutil.which('gotestsum') is not None:
    test_run = str(uuid.uuid4())

    test_results_dir = root.joinpath('test-results')
    if not test_results_dir.is_dir():
        os.mkdir(str(test_results_dir))

    json_file = str(test_results_dir.joinpath(f'{test_run}.json'))
    junit_file = str(test_results_dir.joinpath(f'{test_run}.xml'))
    args = ['gotestsum', '--jsonfile', json_file, '--junitfile', junit_file, '--'] + \
        options_and_packages

    print(' '.join(args))
    if not dryrun:
        sp.check_call(args, shell=False)
else:
    args = ['go', 'test'] + options_and_packages
    print(' '.join(args))
    if not dryrun:
        sp.check_call(args, shell=False)
