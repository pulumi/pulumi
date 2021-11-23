"""Determines if a testsuite is skipped according to
PULUMI_TEST_SUBSET.

"""

from test_subsets import TEST_SUBSETS
import os
import sys


testsuite_name = sys.argv[1]
testsuite_command = sys.argv[2:]
test_subset = os.environ.get('PULUMI_TEST_SUBSET', None)


def should_run():
    if test_subset is None:
        return True

    if test_subset == 'etc':
        s = set([])

        for k in TEST_SUBSETS:
            s = s | set(TEST_SUBSETS[k])

        return testsuite_name not in s

    s = set(TEST_SUBSETS[test_subset])
    return testsuite_name in s


if not should_run():
    print(f'TESTSUITE {testsuite_name} skipped according to PULUMI_TEST_SUBSET={test_subset}')
    sys.exit(0)
else:
    sys.exit(1)
