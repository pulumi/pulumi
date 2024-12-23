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
from os import path, environ
import unittest
from ..util import LanghostTest


class ComponentDependenciesTest(LanghostTest):
    def test_component_dependencies(self):
        environ["PULUMI_ERROR_ON_DEPENDENCY_CYCLES"] = "false"
        self.run_test(
            program=path.join(self.base_path(), "component_dependencies"),
            expected_resource_count=16,
        )
        del environ["PULUMI_ERROR_ON_DEPENDENCY_CYCLES"]

    def register_resource(
        self,
        _ctx,
        _dry_run,
        ty,
        name,
        _resource,
        _dependencies,
        _parent,
        _custom,
        protect,
        _provider,
        _property_deps,
        _delete_before_replace,
        _ignore_changes,
        _version,
        _import,
        _replace_on_changes,
        _providers,
        source_position,
    ):
        if name == "resD":
            self.assertListEqual(_dependencies, ["resA"], msg=f"{name}._dependencies")
            self.assertDictEqual(
                _property_deps,
                {
                    "propA": ["resA"],
                },
                msg=f"{name}._property_deps",
            )
        elif name == "resE":
            self.assertListEqual(_dependencies, ["resD"], msg=f"{name}._dependencies")
            self.assertDictEqual(
                _property_deps,
                {
                    "propA": ["resD"],
                },
                msg=f"{name}._property_deps",
            )
        elif name == "resF":
            self.assertListEqual(_dependencies, ["resA"], msg=f"{name}._dependencies")
            self.assertDictEqual(
                _property_deps,
                {
                    "propA": ["resA"],
                },
                msg=f"{name}._property_deps",
            )
        elif name == "resG":
            self.assertListEqual(
                _dependencies, ["resB", "resD", "resE"], msg=f"{name}._dependencies"
            )
            self.assertDictEqual(
                _property_deps,
                {
                    "propA": ["resB", "resD", "resE"],
                },
                msg=f"{name}._property_deps",
            )
        elif name == "resH":
            self.assertListEqual(
                _dependencies, ["resD", "resE"], msg=f"{name}._dependencies"
            )
            self.assertDictEqual(
                _property_deps,
                {
                    "propA": ["resD", "resE"],
                },
                msg=f"{name}._property_deps",
            )
        elif name == "resI":
            self.assertListEqual(_dependencies, ["resG"], msg=f"{name}._dependencies")
            self.assertDictEqual(
                _property_deps,
                {
                    "propA": ["resG"],
                },
                msg=f"{name}._property_deps",
            )
        elif name == "resJ":
            self.assertListEqual(
                _dependencies, ["resD", "resE"], msg=f"{name}._dependencies"
            )
            self.assertDictEqual(_property_deps, {}, msg=f"{name}._property_deps")
        elif name == "second":
            self.assertListEqual(
                _dependencies, ["firstChild"], msg=f"{name}._dependencies"
            )

        return {
            "urn": name,
            "id": name,
            "object": {
                "outprop": "qux",
            },
        }
