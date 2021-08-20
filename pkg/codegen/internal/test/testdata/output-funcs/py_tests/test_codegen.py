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

from . import funcWithAllOptionalInputs, funcWithDefaultValue, funcWithDictParam, funcWithListParam, \
    getIntegrationRuntimeObjectMetadatum, listStorageAccountKeys

import pulumi_py_tests


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
        if args.token in ['madeup-package:codegentest:funcWithAllOptionalInputs',
                          'madeup-package:codegentest:funcWithDefaultValue']:
            a = args.args.get('a', None)
            b = args.args.get('b', None)
            return {'r': f'a={a} b={b}'}

        if args.token in ['madeup-package:codegentest:funcWithDictParam',
                          'madeup-package:codegentest:funcWithListParam']:
            return {'r': jstr(args.args)}

        if args.token == 'azure-native:codegentest:getIntegrationRuntimeObjectMetadatum':
            return {'nextLink': 'my-next-link',
                    'value': [args.args]}

        if args.token == 'azure-native:codegentest:listStorageAccountKeys':
            return {'keys': [
                dict(creationTime='my-creation-time',
                     keyName='my-key-name',
                     permissions='my-permissions',
                     value=jstr(args.args))
            ]}

        return {}

    def new_resource(self, args):
        return ['', {}]


def assert_function_matches_table(fn, table):

    def check(expected, transform):

        def f(v):
            if transform:
                v = transform(v)

            assert v == expected

        return f

    return pulumi.Output.all([
        fn(**kw).apply(check(expected, transform))
        for (kw, expected, transform) in table
    ])


@pulumi.runtime.test
def test_func_with_all_optional_inputs(my_mocks):
    return assert_function_matches_table(
        funcWithAllOptionalInputs.func_with_all_optional_inputs_output,
        [
            ({}, 'a=None b=None', r),
            ({'a': out('my-a')}, 'a=my-a b=None', r),
            ({'a': out('my-a'), 'b': out('my-b')}, 'a=my-a b=my-b', r),
        ])


@pulumi.runtime.test
def test_func_with_default_value(my_mocks):
    # TODO defaults from schema not recognized
    # https://github.com/pulumi/pulumi/issues/7815
    return assert_function_matches_table(
        funcWithDefaultValue.func_with_default_value_output,
        [
            ({}, 'a=None b=None', r),
            ({'a': out('my-a')}, 'a=my-a b=None', r),
            ({'a': out('my-a'), 'b': out('my-b')}, 'a=my-a b=my-b', r),
        ])


@pulumi.runtime.test
def test_func_with_dict_param(my_mocks):
    d = {'key-a': 'value-a', 'key-b': 'value-b'}
    return assert_function_matches_table(
        funcWithDictParam.func_with_dict_param_output,
        [
            ({}, '{}', r),
            ({'a': out(d)}, jstr({'a': d}), r),
            ({'a': out(d), 'b': out('my-b')}, jstr({'a': d, 'b': 'my-b'}), r),
        ])


@pulumi.runtime.test
def test_func_with_list_param(my_mocks):
    l = ['a', 'b', 'c']
    return assert_function_matches_table(
        funcWithListParam.func_with_list_param_output,
        [
            ({}, '{}', r),
            ({'a': out(l)}, jstr({'a': l}), r),
            ({'a': out(l), 'b': out('my-b')}, jstr({'a': l, 'b': 'my-b'}), r),
        ])


@pulumi.runtime.test
def test_get_integration_runtime_object_metadatum(my_mocks):
    return assert_function_matches_table(
        getIntegrationRuntimeObjectMetadatum.get_integration_runtime_object_metadatum_output,
        [(
            {
                'factory_name': out('my-factory-name'),
                'integration_runtime_name': out('my-integration-runtime-name'),
                'metadata_path': out('metadata-path'),
                'resource_group_name': out('resource-group-name')
            },

            {
                'next_link': 'my-next-link',
                'value': [{
                    'factoryName': 'my-factory-name',
                    'integrationRuntimeName': 'my-integration-runtime-name',
                    'metadataPath': 'metadata-path',
                    'resourceGroupName': 'resource-group-name'
                }],
            },

            lambda r: {'next_link': r.next_link, 'value': r.value}
        )])


@pulumi.runtime.test
def test_list_storage_accounts(my_mocks):
    return assert_function_matches_table(
        listStorageAccountKeys.list_storage_account_keys_output,
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
