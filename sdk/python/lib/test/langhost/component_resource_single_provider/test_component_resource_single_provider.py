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
from os import path
from ..util import LanghostTest


class ComponentResourceSingleProviderTest(LanghostTest):
    """
    Tests that resources inherit a variety of properties from their parents, when parents are present.

    This test generates a multi-level tree of resources of different kinds and parents them in various ways.
    The crux of the test is that all leaf resources in the resource tree should inherit a variety of properties
    from their parents.
    """

    def test_component_resource_single_provider(self):
        self.run_test(
            program=path.join(self.base_path(), "component_resource_single_provider"),
            expected_resource_count=240)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if _custom and not ty.startswith("pulumi:providers:"):
            expect_protect = False
            expect_provider_name = ""

            rpath = name.split("/")
            for i, component in enumerate(rpath[1:]):
                if component in ["r0", "c0"]:
                    # No overrides - get all values from parent.
                    continue
                if component in ["r1", "c1"]:
                    # Protect overriden to be false.
                    expect_protect = False
                    continue
                if component in ["r2", "c2"]:
                    # Protect overriden to be true.
                    expect_protect = True
                    continue
                if component in ["r3", "c3"]:
                    # Provider overriden.
                    expect_provider_name = "/".join(rpath[:i+1]) + "-p"

            # r3 explicitly overrides its provider.
            if rpath[-1] == "r3":
                expect_provider_name = "/".join(rpath[:-1]) + "-p"

            # "provider" is a provider reference. To get the provider name (technically its ID, but this test
            # uses names as IDs), get the first string before the double-colon.
            provider_name = _provider.split("::")[-1]
            self.assertEqual(f"{name}.protect: {protect}", f"{name}.protect: {expect_protect}")
            self.assertEqual(f"{name}.provider: {provider_name}", f"{name}.provider: {expect_provider_name}")

        return {
            "urn": self.make_urn(ty, name),
            "id": name,
        }
