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

import pulumi

@pulumi.input_type
class MyResourceArgs:
    def __init__(self, policy: pulumi.Input[str]):
        pulumi.set(self, "policy", policy)

    @property
    @pulumi.getter
    def policy(self) -> pulumi.Input[str]:
        return pulumi.get(self, "policy")

class MyResource(pulumi.CustomResource):
    def __init__(self, name, args: MyResourceArgs):
        super().__init__("test:index:MyResource", name, args)

    @property
    @pulumi.getter
    def policy(self) -> pulumi.Output[str]:
        return pulumi.get(self, "policy")

r1 = MyResource("testResource", MyResourceArgs(policy='{"hello": "world"}'))

# We have cases where an input is typed as `Input[str]` but passing a dict still works despite
# the type annotation not saying that it accepts a dict (e.g. aws.s3.BucketPolicy.policy, which
# is typed as `Input[str]` but passing a dict works because the provider will convert the dict
# to a JSON string). We need to fix the type annotations in these cases, but in the meantime such
# cases should continue to work as before without an error being raised by the SDK.
r2 = MyResource("testResource", MyResourceArgs(policy={"hello": "world"}))

pulumi.export("r1.policy", r1.policy)
pulumi.export("r2.policy", r2.policy)
