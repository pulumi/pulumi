"""
Wraps `go test`.
"""

from test_subsets import TEST_SUBSETS
import os
import pathlib
import shutil
import subprocess as sp
import sys
import uuid


def options(options_and_packages):
    return [o for o in options_and_packages if '/' not in o]


def packages(options_and_packages):
    return [o for o in options_and_packages if '/' in o]


def filter_packages(packages, test_subset=None):
    if test_subset is None:
        return packages

    if test_subset == 'etc':
        s = set([])
        for k in TEST_SUBSETS:
            s = s | set(TEST_SUBSETS[k])
        return [p for p in packages if p not in s]

    s = set(TEST_SUBSETS[test_subset])
    return [p for p in packages if p in s]


root = pathlib.Path(__file__).absolute().parent.parent
test_subset = os.environ.get('PULUMI_TEST_SUBSET', None)
options_and_packages = sys.argv[1:]
packages = filter_packages(packages(options_and_packages), test_subset=test_subset)


if not packages:
    print(f'No packages matching PULUMI_TEST_SUBSET={test_subset}')
    sys.exit(0)


options = options(options_and_packages)
cov = os.environ.get('PULUMI_TEST_COVERAGE_PATH', None)
if cov is not None:
    options = options + [f'-coverprofile={cov}/go-test-{os.urandom(4).hex()}.cov', '-coverpkg=github.com/pulumi/pulumi/pkg/v3/...,github.com/pulumi/pulumi/sdk/v3/...']


options_and_packages = options + packages


if shutil.which('gotestsum') is not None:
    test_run = str(uuid.uuid4())

    test_results_dir = root.joinpath('test-results')
    if not test_results_dir.is_dir():
        os.mkdir(str(test_results_dir))

    json_file = str(test_results_dir.joinpath(f'{test_run}.json'))
    junit_file = str(test_results_dir.joinpath(f'{test_run}.xml'))
    sp.check_call(['gotestsum', '--jsonfile', json_file, '--junitfile', junit_file, '--'] + \
                  options_and_packages, shell=False)
else:
    sp.check_call(['go', 'test'] + options_and_packages, shell=False)
