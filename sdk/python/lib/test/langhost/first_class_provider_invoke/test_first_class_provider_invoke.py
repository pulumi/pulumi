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


class TestFirstClassProviderInvoke(LanghostTest):
    """
    Tests that Invoke passes provider references to the engine, both with and without the presence of a "parent" from
    which to derive a provider.
    """
    def setUp(self):
        self.prov_id = None
        self.prov_urn = None

    def test_first_class_provider_invoke(self):
        self.run_test(
            program=path.join(self.base_path(), "first_class_provider_invoke"),
            expected_resource_count=4)

    def invoke(self, _ctx, token, args, provider, _version):
        # MyFunction explicitly receives a provider reference.
        if token == "test:index:MyFunction":
            self.assertDictEqual({
                "value": 9000,
            }, args)
            self.assertEqual(f"{self.prov_urn}::{self.prov_id}", provider)
        # MyFunctionWithParent implicitly receives a provider reference because it is the child of a resource that
        # overrides the provider for "test".
        elif token == "test:index:MyFunctionWithParent":
            self.assertDictEqual({
                "value": 41
            }, args)
            self.assertEqual(f"{self.prov_urn}::{self.prov_id}", provider)
        else:
            self.fail(f"unexpected token: {token}")

        # Return the value + 1. This value is roundtripped to `register_resource` below.
        return [], {
            "value": args["value"] + 1
        }

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if name == "testprov":
            self.assertEqual("pulumi:providers:test", ty)
            self.prov_urn = self.make_urn(ty, name)
            self.prov_id = name
            return {
                "urn": self.prov_urn,
                "id": self.prov_id,
            }

        if name == "resourceA":
            self.assertEqual("test:index:MyResource", ty)
            self.assertEqual(_resource["value"], 9001)
            return {
                "urn": self.make_urn(ty, name),
                "id": name,
                "object": _resource,
            }

        if name == "resourceB":
            self.assertEqual("test:index:MyComponent", ty)
            return {
                "urn": self.make_urn(ty, name),
            }

        if name == "resourceC":
            self.assertEqual("test:index:MyResource", ty)
            self.assertEqual(_resource["value"], 42)
            return {
                "urn": self.make_urn(ty, name),
                "id": name,
                "object": _resource,
            }

        self.fail(f"unexpected resource: {name}")
