# Copyright 2016-2023, Pulumi Corporation.
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


class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + '_id', args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        assert args.token == "test:index:MyFunction"
        return {}


@pulumi.runtime.test
def test_invoke_empty_return() -> None:
    pulumi.runtime.mocks.set_mocks(MyMocks())

    ret = pulumi.runtime.invoke("test:index:MyFunction", {})
    assert ret.value == {}, "Expected the return value of the invoke to be an empty dict"


class MyKubernetesMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + '_id', args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        assert args.token in {
            "kubernetes:yaml:decode",
            "kubernetes:helm:template",
            "kubernetes:kustomize:directory",
        }
        return {"result": "mock"} if args.args.get("nonempty") else {}


# Regression test for https://github.com/pulumi/pulumi/issues/14508.
@pulumi.runtime.test
def test_invoke_kubernetes() -> None:
    pulumi.runtime.mocks.set_mocks(MyKubernetesMocks())

    # Invokes to these specific Kubernetes functions return None rather than an empty dict for empty results.
    assert pulumi.runtime.invoke("kubernetes:yaml:decode", {}).value is None
    assert pulumi.runtime.invoke("kubernetes:helm:template", {}).value is None
    assert pulumi.runtime.invoke("kubernetes:kustomize:directory", {}).value is None

    # Non-empty results are returned as-is.
    assert pulumi.runtime.invoke("kubernetes:yaml:decode", {"nonempty": True}).value == {"result": "mock"}
    assert pulumi.runtime.invoke("kubernetes:helm:template", {"nonempty": True}).value == {"result": "mock"}
    assert pulumi.runtime.invoke("kubernetes:kustomize:directory", {"nonempty": True}).value == {"result": "mock"}
