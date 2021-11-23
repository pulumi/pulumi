"""Determines if a testsuite is skipped according to
`PULUMI_TEST_SUBSET` or `--skip-windows`.

"""

from test_subsets import TEST_SUBSETS
import os
import sys

args = sys.argv[1:]

skip_windows = False
if args and args[0] == '--skip-windows':
    args = args[1:]
    skip_windows = True

testsuite_name = args[0]
testsuite_command = args[1:]
test_subset = os.environ.get('PULUMI_TEST_SUBSET', None)


def get_skip_reason():

    if skip_windows and os.name == 'nt':
        return 'on Windows'

    if test_subset is None:
        return None # do not skip

    if test_subset == 'etc':

        matching_subset = None

        for k in TEST_SUBSETS:
            if testsuite_name in TEST_SUBSETS[k]:
                matching_subset = k

        if matching_subset is not None:
            return f'in PULUMI_TEST_SUBSET={matching_subset}, not etc'

        return None # do not skip

    if testsuite_name not in set(TEST_SUBSETS[test_subset]):
        return f'not in PULUMI_TEST_SUBSET={test_subset}'

    return None # do no skip


skip_reason = get_skip_reason()
if skip_reason is not None:
    print(f'TESTSUITE {testsuite_name} skipped: {skip_reason}')
    sys.exit(0)
else:
    sys.exit(1)
