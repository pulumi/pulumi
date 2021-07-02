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


class FirstClassProviderTest(LanghostTest):
    """
    Tests that resources created with their 'provider' ResourceOption set pass a provider reference
    to the Pulumi engine.
    """
    prov_urn = None
    prov_id = None

    def test_first_class_provider(self):
        self.run_test(
            program=path.join(self.base_path(), "first_class_provider"),
            expected_resource_count=2)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if name == "testprov":
            # Provider resource.
            self.assertEqual("pulumi:providers:test", ty)
            self.assertEqual("", _provider)
            self.prov_urn = self.make_urn(ty, name)
            self.prov_id = "testid"
            return {
                "urn": self.prov_urn,
                "id": self.prov_id
            }

        if name == "testres":
            # Test resource using the provider.
            self.assertEqual("test:index:Resource", ty)
            self.assertIsNotNone(self.prov_urn)
            self.assertIsNotNone(self.prov_id)

            # The provider reference is created by concatenating the URN and ID of the referenced provider.
            # The language host is responsible for doing this, since the engine will parse this identifier.
            self.assertEqual(f"{self.prov_urn}::{self.prov_id}", _provider)
            return {
                "urn": self.make_urn(ty, name)
            }

        self.fail(f"unexpected resource: {name} ({ty})")
