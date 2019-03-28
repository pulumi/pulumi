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
import unittest
from ..util import LanghostTest


class InvalidPropertyDependencyTest(LanghostTest):
    def test_invalid_property_dependency(self):
        self.run_test(
            program=path.join(self.base_path(), "invalid_property_dependency"),
            expected_error="Program exited with non-zero exit code: 1",
            expected_resource_count=1)

    def register_resource(self, _ctx, _dry_run, ty, name, resource,
                          _dependencies, _parent, _custom, _protect, _provider, _property_dependencies, _delete_before_replace):
        self.assertEqual(ty, "test:index:MyResource")
        if name == "resA":
            self.assertListEqual(_dependencies, [])
            self.assertDictEqual(_property_dependencies, {})
        else:
            self.fail(f"unexpected resource: {name} ({ty})")

        return {
            "urn": name,
            "id": name,
            "object": {
                "outprop": "qux",
            }
        }
