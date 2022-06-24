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


class OutputNestedTest(LanghostTest):
    def test_output_nested(self):
        self.run_test(
            program=path.join(self.base_path(), "output_nested"),
            expected_resource_count=3)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        nested_numbers = None
        if name == "testResource1":
            self.assertEqual(ty, "test:index:MyResource")
            nested_numbers = {
                "foo": {
                    "bar": 9
                },
                "baz": 1,
            }
        elif name == "testResource2":
            self.assertEqual(ty, "test:index:MyResource")
            nested_numbers = {
                "foo": {
                    "bar": 99,
                },
                "baz": 1,
            }
        elif name == "sumResource":
            self.assertEqual(ty, "test:index:SumResource")
            # The source program uses Output.apply to merge outputs from the above two resources.
            # The 10 is produced by adding 9 and 1 in the source program, derived from nested properties of the
            # testResource1 nested_numbers property.
            self.assertEqual(_resource["sum"], 10)
            nested_numbers = _resource["sum"]
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": {
                "nested_numbers": nested_numbers
            }
        }
