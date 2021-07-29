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
import unittest
from ..util import LanghostTest


class PropertyDependenciesTest(LanghostTest):
    def test_property_dependencies(self):
        self.run_test(
            program=path.join(self.base_path(), "property_dependencies"),
            expected_resource_count=5)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        self.assertEqual(ty, "test:index:MyResource")
        if name == "resA":
            self.assertListEqual(_dependencies, [])
            self.assertDictEqual(_property_deps, {})
        elif name == "resB":
            self.assertListEqual(_dependencies, [ "resA" ])
            self.assertDictEqual(_property_deps, {})
        elif name == "resC":
            self.assertListEqual(_dependencies, [ "resA", "resB" ])
            self.assertDictEqual(_property_deps, {
                "propA": [ "resA" ],
                "propB": [ "resB" ],
                "propC": [],
            })
        elif name == "resD":
            self.assertListEqual(_dependencies, [ "resA", "resB", "resC" ])
            self.assertDictEqual(_property_deps, {
                "propA": [ "resA", "resB" ],
                "propB": [ "resC" ],
                "propC": [],
            })
        elif name == "resE":
            self.assertListEqual(_dependencies, [ "resA", "resB", "resC", "resD" ])
            self.assertDictEqual(_property_deps, {
                "propA": [ "resC" ],
                "propB": [ "resA", "resB" ],
                "propC": [],
            })

        return {
            "urn": name,
            "id": name,
            "object": {
                "outprop": "qux",
            }
        }
