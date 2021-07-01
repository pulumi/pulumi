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


class TestTypes(LanghostTest):
    def test_types(self):
        self.run_test(
            program=path.join(self.base_path(), "types"),
            expected_resource_count=16)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if name in ["testres", "testres2", "testres3", "testres4"]:
            self.assertIn("additional", _resource)
            self.assertEqual({
                "firstValue": "hello",
                "secondValue": 42,
            }, _resource["additional"])
        elif name in ["testres5", "testres6", "testres7", "testres8"]:
            self.assertIn("extra", _resource)
            self.assertEqual({
                "firstValue": "foo",
                "secondValue": 100,
            }, _resource["extra"])
        elif name in ["testres9", "testres10", "testres11", "testres12"]:
            self.assertIn("supplementary", _resource)
            self.assertEqual({
                "firstValue": "bar",
                "secondValue": 200,
                "third": "third value",
                "fourth": "fourth value",
            }, _resource["supplementary"])
        elif name in ["testres13", "testres14", "testres15", "testres16"]:
            self.assertIn("ancillary", _resource)
            self.assertEqual({
                "firstValue": "baz",
                "secondValue": 500,
                "third": "third value!",
                "fourth": "fourth!",
            }, _resource["ancillary"])
        else:
            self.fail(f"unknown resource: {name}")
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": _resource,
        }
