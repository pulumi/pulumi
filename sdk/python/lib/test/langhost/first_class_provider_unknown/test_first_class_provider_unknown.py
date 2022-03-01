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
from pulumi.runtime import rpc
from ..util import LanghostTest


class FirstClassProviderUnknown(LanghostTest):
    """
    Tests that when a first class provider's ID isn't known in a preview, the language host passes a provider reference
    to the engine using the rpc UNKNOWN sentinel in place of the ID.
    """
    def setUp(self):
        self.prov_id = None
        self.prov_urn = None

    def test_first_class_provider_unknown(self):
        self.run_test(
            program=path.join(self.base_path(), "first_class_provider_unknown"),
            expected_resource_count=2)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if name == "testprov":
            self.assertEqual("pulumi:providers:test", ty)
            # Only provide an ID when doing an update. When doing a preview the ID will be unknown
            # and resources referencing this resource will need to use the unknown sentinel to do so.
            self.prov_urn = self.make_urn(ty, name)
            if _dry_run:
                return {
                    "urn": self.prov_urn
                }

            self.prov_id = name
            return {
                "urn": self.prov_urn,
                "id": self.prov_id
            }

        if name == "res":
            self.assertEqual("test:index:MyResource", ty)
            if _dry_run:
                # During a preview, the ID of the pulumi:providers:test resource is unknown.
                self.assertEqual(f"{self.prov_urn}::{rpc.UNKNOWN}", _provider)
            else:
                # Otherwise, it's known to be exactly the above provider's ID.
                self.assertEqual(f"{self.prov_urn}::{self.prov_id}", _provider)
            return {
                "urn": self.make_urn(ty, name),
                "id": name,
                "object": _resource
            }

        self.fail(f"unknown resource: {name} ({ty})")
