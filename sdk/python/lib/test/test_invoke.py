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

import pytest

import pulumi


class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {} if args.args.get("empty") else {"result": "mock"}


@pytest.mark.parametrize(
    "tok,version,empty,expected",
    [
        ("test:index:MyFunction", "", True, {}),
        ("test:index:MyFunction", "invalid", True, {}),
        ("test:index:MyFunction", "1.0.0", True, {}),
        ("test:index:MyFunction", "", False, {"result": "mock"}),
        ("test:index:MyFunction", "invalid", False, {"result": "mock"}),
        ("test:index:MyFunction", "1.0.0", False, {"result": "mock"}),
        ("kubernetes:something:new", "", True, {}),
        ("kubernetes:something:new", "invalid", True, {}),
        ("kubernetes:something:new", "1.0.0", True, {}),
        ("kubernetes:something:new", "4.5.3", True, {}),
        ("kubernetes:something:new", "4.5.4", True, {}),
        ("kubernetes:something:new", "4.5.5", True, {}),
        ("kubernetes:something:new", "4.6.0", True, {}),
        ("kubernetes:something:new", "5.0.0", True, {}),
        ("kubernetes:something:new", "", False, {"result": "mock"}),
        ("kubernetes:something:new", "invalid", False, {"result": "mock"}),
        ("kubernetes:something:new", "1.0.0", False, {"result": "mock"}),
        # Expect the legacy behavior of getting None for empty results for these Kubernetes
        # function tokens when the version is unspecified, invalid, or <= 4.5.4.
        ("kubernetes:yaml:decode", "", True, None),
        ("kubernetes:yaml:decode", "invalid", True, None),
        ("kubernetes:yaml:decode", "1.0.0", True, None),
        ("kubernetes:yaml:decode", "4.5.3", True, None),
        ("kubernetes:yaml:decode", "4.5.4", True, None),
        ("kubernetes:yaml:decode", "4.5.5", True, {}),
        ("kubernetes:yaml:decode", "4.6.0", True, {}),
        ("kubernetes:yaml:decode", "5.0.0", True, {}),
        ("kubernetes:yaml:decode", "", False, {"result": "mock"}),
        ("kubernetes:yaml:decode", "invalid", False, {"result": "mock"}),
        ("kubernetes:yaml:decode", "1.0.0", False, {"result": "mock"}),
        ("kubernetes:helm:template", "", True, None),
        ("kubernetes:helm:template", "invalid", True, None),
        ("kubernetes:helm:template", "1.0.0", True, None),
        ("kubernetes:helm:template", "4.5.3", True, None),
        ("kubernetes:helm:template", "4.5.4", True, None),
        ("kubernetes:helm:template", "4.5.5", True, {}),
        ("kubernetes:helm:template", "4.6.0", True, {}),
        ("kubernetes:helm:template", "5.0.0", True, {}),
        ("kubernetes:helm:template", "", False, {"result": "mock"}),
        ("kubernetes:helm:template", "invalid", False, {"result": "mock"}),
        ("kubernetes:helm:template", "1.0.0", False, {"result": "mock"}),
        ("kubernetes:kustomize:directory", "", True, None),
        ("kubernetes:kustomize:directory", "invalid", True, None),
        ("kubernetes:kustomize:directory", "1.0.0", True, None),
        ("kubernetes:kustomize:directory", "4.5.3", True, None),
        ("kubernetes:kustomize:directory", "4.5.4", True, None),
        ("kubernetes:kustomize:directory", "4.5.5", True, {}),
        ("kubernetes:kustomize:directory", "4.6.0", True, {}),
        ("kubernetes:kustomize:directory", "5.0.0", True, {}),
        ("kubernetes:kustomize:directory", "", False, {"result": "mock"}),
        ("kubernetes:kustomize:directory", "invalid", False, {"result": "mock"}),
        ("kubernetes:kustomize:directory", "1.0.0", False, {"result": "mock"}),
    ],
)
@pulumi.runtime.test
def test_invoke_empty_return(tok: str, version: str, empty: bool, expected) -> None:
    pulumi.runtime.mocks.set_mocks(MyMocks())

    props = {"empty": True} if empty else {}
    opts = pulumi.InvokeOptions(version=version) if version else None
    assert pulumi.runtime.invoke(tok, props, opts).value == expected
