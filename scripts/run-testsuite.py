"""
Wraps test suite invocation shell commands to measure time and
provide awareness of test configurations set by PULUMI_TEST_SUBSET.
"""

from test_subsets import TEST_SUBSETS
import os
import subprocess as sp
import sys
import timeit


testsuite_name = sys.argv[1]
testsuite_command = sys.argv[2:]
test_subset = os.environ.get('PULUMI_TEST_SUBSET', None)


def should_run():
    if test_subset is None:
        return True

    if test_subset == 'etc':
        s = set([])

        for k in TEST_SUBSETS:
            s = s | set(TEST_SUBSETS[k].test_suites)

        return testsuite_name not in s

    s = set(TEST_SUBSETS[test_subset].test_suites)
    return testsuite_name in s


if not should_run():
    print(f'TESTSUITE {testsuite_name} skipped according to PULUMI_TEST_SUBSET={test_subset}')
    sys.exit(0)


t0 = timeit.timeit()
try:
    sp.check_call(testsuite_command, shell=False)
finally:
    t1 = timeit.timeit()
    elapsed = t1 - t0
    print(f'TESTSUITE {testsuite_name} completed in {elapsed}')
