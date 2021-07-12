"""Reads json files produced by `go test -json` from
`$PULUMI_ROOT/test-results` and summarizes cumulative package
execution time and longest executing tests.

"""

import datetime
import json
import pathlib
import os
import os.path
import re

PULUMI_ROOT = pathlib.Path(__file__).parent.resolve().parent
TEST_RESULTS = os.path.join(PULUMI_ROOT, "test-results")


def parse_time(json_line):
    time_str = json_line['Time']
    pat = re.compile(r"(\d+)(?:Z|[-]?\d+[:]\d+)$")
    m = pat.search(time_str)
    if m:
        time_str = pat.sub(str(100 * round(int(m.groups(1)[0]) / 1000)), time_str)
    return datetime.datetime.strptime(time_str, '%Y-%m-%dT%H:%M:%S.%f')


def dur(t):
    return (t['finished'] - t['started']).total_seconds()


def read_test_info(fp, test_info):
    for line in fp:
        if line.strip():
            datum = json.loads(line)

            if datum['Action'] == 'run':
                test_info[(datum['Package'], datum['Test'])] = {'started': parse_time(datum)}

            elif datum['Action'] == 'pass' and 'Test' in datum:
                t = test_info[(datum['Package'], datum['Test'])]
                t['finished'] = parse_time(datum)
                t['result'] = 'pass'


def package_durations(test_info):
    pd = {}
    for ((pkg, test), t) in test_info.items():
        pd[pkg] = pd.get(pkg, 0) + dur(t)
    return pd


def slowest_tests(test_info):
    return [
        (pkg, test, dur(t))
        # (pkg, test)
        # for ((pkg, test), _)
        for ((pkg, test), t) in sorted(test_info.items(), key=lambda x: -dur(x[1]))
    ]


test_info = {}
for filepath in os.listdir(TEST_RESULTS):
    if filepath.endswith('.json'):
        with open(os.path.join(TEST_RESULTS, filepath), 'r') as fp:
            read_test_info(fp, test_info)


pkg_durations = package_durations(test_info)


print('# packages by cumulative test execution time:')
for pkg, pkg_dur in sorted(pkg_durations.items(), key=lambda kv: -kv[1]):
    print(f'{pkg}: {pkg_dur} sec')
print()

print('# slowest tests:')
for pkg, test, test_dur in slowest_tests(test_info):
    print(f'{pkg} {test}: {test_dur} sec')
print()
