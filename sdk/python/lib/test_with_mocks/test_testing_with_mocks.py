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

from concurrent.futures import ThreadPoolExecutor
import asyncio
import grpc
import pulumi
import pytest
import resources
import unittest


@pytest.fixture
def my_resources():
    loop = asyncio.get_event_loop()
    loop.set_default_executor(ImmediateExecutor())

    old_settings = pulumi.runtime.settings.SETTINGS
    try:
        pulumi.runtime.mocks.set_mocks(MyMocks())
        yield resources.define_resources()
    finally:
        pulumi.runtime.settings.configure(old_settings)
        loop.set_default_executor(ThreadPoolExecutor())


@pulumi.runtime.test
def test_component(my_resources):

    def check_outprop(outprop):
        assert outprop == 'output: hello'

    return my_resources['mycomponent'].outprop.apply(check_outprop)


@pulumi.runtime.test
def test_remote_component(my_resources):

    def check_outprop(outprop):
        assert outprop.startswith("output: hello: ")

    return my_resources['myremotecomponent'].outprop.apply(check_outprop)


@pulumi.runtime.test
def test_custom(my_resources):

    def check_ip(ip):
        assert ip == '203.0.113.12'

    return my_resources['myinstance'].public_ip.apply(check_ip)


@pulumi.runtime.test
def test_custom_resource_reference(my_resources):

    def check_instance(instance):
        assert isinstance(instance, resources.Instance)

        def check_ip(ip):
            assert ip == '203.0.113.12'

        instance.public_ip.apply(check_ip)

    return my_resources['mycustom'].instance.apply(check_instance)


@pulumi.runtime.test
def test_invoke(my_resources):
    assert my_resources['invoke_result'] == 59


@pulumi.runtime.test
def test_invoke_failures(my_resources):
    caught = False

    try:
        pulumi.runtime.invoke("test:index:FailFunction", props={})
    except Exception as e:
        caught = str(e)

    assert 'this function fails!' in caught


@pulumi.runtime.test
def test_invoke_throws(my_resources):
    caught = None

    try:
        pulumi.runtime.invoke("test:index:ThrowFunction", props={})
    except Exception as e:
        caught = str(e)

    assert 'this function throws!' in caught


@pulumi.runtime.test
def test_stack_reference(my_resources):

    def check_outputs(outputs):
        assert outputs["haha"] == "business"

    my_resources['dns_ref'].outputs.apply(check_outputs)



class GrpcError(grpc.RpcError):
    def __init__(self, code, details):
        self._code = code
        self._details = details

    def code(self):
        return self._code

    def details(self):
        return self._details


class MyMocks(pulumi.runtime.Mocks):
    def call(self, args: pulumi.runtime.MockCallArgs):
        if args.token == 'test:index:MyFunction':
            return {
                'out_value': 59,
            }
        elif args.token == 'test:index:FailFunction':
            return ({}, [('none', 'this function fails!')])
        elif args.token == 'test:index:ThrowFunction':
            raise GrpcError(42, 'this function throws!')
        else:
            return {}

    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        if args.typ == 'aws:ec2/securityGroup:SecurityGroup':
            state = {
                'arn': 'arn:aws:ec2:us-west-2:123456789012:security-group/sg-12345678',
                'name': args.inputs['name'] if 'name' in args.inputs else args.name + '-sg',
            }
            return ['sg-12345678', dict(args.inputs, **state)]
        elif args.typ == 'aws:ec2/instance:Instance':
            state = {
                'arn': 'arn:aws:ec2:us-west-2:123456789012:instance/i-1234567890abcdef0',
                'instanceState': 'running',
                'primaryNetworkInterfaceId': 'eni-12345678',
                'privateDns': 'ip-10-0-1-17.ec2.internal',
                'public_dns': 'ec2-203-0-113-12.compute-1.amazonaws.com',
                'public_ip': '203.0.113.12',
            }
            return ['i-1234567890abcdef0', dict(args.inputs, **state)]
        elif args.typ == 'pkg:index:MyCustom':
            return [args.name + '_id', args.inputs]
        elif args.typ == 'pulumi:pulumi:StackReference' and 'dns' in args.name:
            return [args.name, {'outputs': {'haha': 'business'}}]
        elif args.typ == 'pkg:index:MyRemoteComponent':
            state = {
                'outprop': f"output: {args.inputs['inprop']}",
            }
            return [args.name + '_id', dict(args.inputs, **state)]
        else:
            return ['', {}]


class ImmediateExecutor(ThreadPoolExecutor):
    """This removes multithreading from current tests. Unfortunately in
    presence of multithreading the tests are flaky. The proper fix is
    postponed - see https://github.com/pulumi/pulumi/issues/7663

    """

    def __init__(self):
        super()
        self._default_executor = ThreadPoolExecutor()

    def submit(self, fn, *args, **kwargs):
        v = fn(*args, **kwargs)
        return self._default_executor.submit(ImmediateExecutor._identity, v)

    def map(self, func, *iterables, timeout=None, chunksize=1):
        raise Exception('map not implemented')

    def shutdown(self, wait=True, cancel_futures=False):
        raise Exception('shutdown not implemented')

    @staticmethod
    def _identity(x):
        return x
