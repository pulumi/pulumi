# Copyright 2016-2018, Pulumi Corporation.
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
import unittest
import pulumi
import grpc


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
        else:
            return ['', {}]


pulumi.runtime.set_mocks(MyMocks())

# Now actually import the code that creates resources, and then test it.
import resources


class TestingWithMocks(unittest.TestCase):
    @unittest.skip(reason="Skipping flaky test tracked in https://github.com/pulumi/pulumi/issues/6561")
    @pulumi.runtime.test
    def test_component(self):
        def check_outprop(outprop):
            self.assertEqual(outprop, 'output: hello')
        return resources.mycomponent.outprop.apply(check_outprop)

    @pulumi.runtime.test
    def test_custom(self):
        def check_ip(ip):
            self.assertEqual(ip, '203.0.113.12')
        return resources.myinstance.public_ip.apply(check_ip)

    @pulumi.runtime.test
    def test_custom_resource_reference(self):
        def check_instance(instance):
            self.assertIsInstance(instance, resources.Instance)
            def check_ip(ip):
                self.assertEqual(ip, '203.0.113.12')
            instance.public_ip.apply(check_ip)
        return resources.mycustom.instance.apply(check_instance)

    @pulumi.runtime.test
    def test_invoke(self):
        return self.assertEqual(resources.invoke_result, 59)

    @pulumi.runtime.test
    def test_invoke_failures(self):
        caught = False
        try:
            pulumi.runtime.invoke("test:index:FailFunction", props={})
        except Exception:
            caught = True
        self.assertTrue(caught)

    @pulumi.runtime.test
    def test_invoke_throws(self):
        caught = False
        try:
            pulumi.runtime.invoke("test:index:ThrowFunction", props={})
        except Exception:
            caught = True
        self.assertTrue(caught)

    @pulumi.runtime.test
    def test_stack_reference(self):
        def check_outputs(outputs):
            self.assertEqual(outputs["haha"], "business")
        resources.dns_ref.outputs.apply(check_outputs)
