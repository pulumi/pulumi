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


class ResourceThensTest(LanghostTest):
    """
    Test that tests Pulumi's ability to track dependencies between resources.

    ResourceA has an (unknown during preview) output property that ResourceB
    depends on. In all cases, the SDK must inform the engine that ResourceB
    depends on ResourceA. When not doing previews, ResourceB has a partial view
    of ResourceA's properties. 
    """
    def test_resource_thens(self):
        self.run_test(
            program=path.join(self.base_path(), "resource_thens"),
            expected_resource_count=2)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if ty == "test:index:ResourceA":
            self.assertEqual(name, "resourceA")
            self.assertDictEqual(_resource, {"inprop": 777, "inprop_2": 42})
            urn = self.make_urn(ty, name)
            res_id = ""
            props = {}
            if not _dry_run:
                res_id = name
                props["outprop"] = "output yeah"

            return {
                "urn": urn,
                "id": res_id,
                "object": props
            }

        if ty == "test:index:ResourceB":
            self.assertEqual(name, "resourceB")
            self.assertListEqual(_dependencies, ["test:index:ResourceA::resourceA"])
            if _dry_run:
                self.assertDictEqual(_resource, {
                    # other_in is unknown, so it is not in the dictionary.
                    # other_out is unknown, so it is not in the dictionary.
                    # other_id is also unknown so it is not in the dictionary
                })
            else:
                self.assertDictEqual(_resource, {
                    "other_in": 777,
                    "other_out": "output yeah",
                    "other_id": "resourceA",
                })

            res_id = ""
            if not _dry_run:
                res_id = name

            return {
                "urn": self.make_urn(ty, name),
                "id": res_id,
                "object": {}
            }

        self.fail(f"unknown resource type: {ty}")
