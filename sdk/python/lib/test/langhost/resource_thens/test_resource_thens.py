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

    def register_resource(self, _ctx, dry_run, ty, name, res, deps):
        if ty == "test:index:ResourceA":
            self.assertEqual(name, "resourceA")
            self.assertDictEqual(res, {"inprop": 777})
            urn = self.make_urn(ty, name)
            res_id = None
            props = {}
            if not dry_run:
                res_id = name
                props["outprop"] = "output yeah"

            return {
                "urn": urn,
                "id": res_id,
                "object": props
            }

        if ty == "test:index:ResourceB":
            self.assertEqual(name, "resourceB")
            self.assertListEqual(deps, ["test:index:ResourceA::resourceA"])
            if dry_run:
                self.assertDictEqual(res, {
                    "other_in": 777,
                    # other_out is unknown, so it is not in the dictionary.
                })
            else:
                self.assertDictEqual(res, {
                    "other_in": 777,
                    "other_out": "output yeah"
                })

            res_id = None
            if not dry_run:
                res_id = name

            return {
                "urn": self.make_urn(ty, name),
                "id": res_id,
                "object": {}
            }

        self.fail(f"unknown resource type: {ty}")
