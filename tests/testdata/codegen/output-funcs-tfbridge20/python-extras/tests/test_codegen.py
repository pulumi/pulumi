# Copyright 2016-2021, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


import json
import pytest

import pulumi

from pulumi_mypkg import *


@pytest.fixture
def my_mocks():
    old_settings = pulumi.runtime.settings.SETTINGS
    try:
        mocks = MyMocks()
        pulumi.runtime.mocks.set_mocks(mocks)
        yield mocks
    finally:
        pulumi.runtime.settings.configure(old_settings)


class MyMocks(pulumi.runtime.Mocks):
    def call(self, args):

        if args.token == 'mypkg::listStorageAccountKeys':
            return {'keys': [
                dict(creationTime='my-creation-time',
                     keyName='my-key-name',
                     permissions='my-permissions',
                     value=jstr(args.args))
            ]}

        raise Exception(f'Unhandled args.token={args.token}')

    def new_resource(self, args):
        return ['', {}]


def assert_function_matches_table(fn, table):

    def check(expected, transform):

        def f(v):
            if transform:
                v = transform(v)

            assert v == expected

        return f

    def unpack_entry(entry):
        if len(entry) == 3:
            (kw, expected, transform) = entry
            args = []
        else:
            (args, kw, expected, transform) = entry

        return (args, kw, expected, transform)

    return pulumi.Output.all([
        fn(*args, **kw).apply(check(expected, transform))
        for (args, kw, expected, transform) in (
                unpack_entry(entry) for entry in table
        )
    ])


@pulumi.runtime.test
def test_list_storage_accounts(my_mocks):
    return assert_function_matches_table(list_storage_account_keys_output,
        [(
            {
                'account_name': out('my-account-name'),
                'expand': out('my-expand'),
                'resource_group_name': out('my-resource-group-name'),
            },

            {
                'creation_time': 'my-creation-time',
                'key_name': 'my-key-name',
                'permissions': 'my-permissions',
                'value': jstr({
                    'accountName': 'my-account-name',
                    'expand': 'my-expand',
                    'resourceGroupName': 'my-resource-group-name',
                })
            },

            lambda r: {
                'creation_time': r.keys[0].creation_time,
                'key_name': r.keys[0].key_name,
                'permissions': r.keys[0].permissions,
                'value': r.keys[0].value,
            }
        )])


def jstr(x):
    return json.dumps(x, sort_keys=True)


def r(x):
    return x.r


def out(x):
    return pulumi.Output.from_input(x).apply(lambda x: x)
