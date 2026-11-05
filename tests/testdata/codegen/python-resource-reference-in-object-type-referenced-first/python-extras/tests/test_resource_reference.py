# Copyright 2026, Pulumi Corporation.
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


class Mocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


pulumi.runtime.set_mocks(Mocks())

import pulumi_example as example  # noqa: E402


@pulumi.runtime.test
def test_referenced_resource_loads_first():
    alpha = example.Alpha("alpha", settings=example.AlphaSettingsArgs(name="a"))
    beta = example.Beta("beta", alpha_ref=example.AlphaRefArgs(alpha=alpha))
    return pulumi.Output.all(alpha.urn, beta.urn)
