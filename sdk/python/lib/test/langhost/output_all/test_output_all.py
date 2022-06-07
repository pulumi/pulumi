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


class OutputAllTest(LanghostTest):
    """
    """
    def test_output_all(self):
        self.run_test(
            program=path.join(self.base_path(), "output_all"),
            expected_resource_count=4)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        number = 0
        if name == "testResource1":
            self.assertEqual(ty, "test:index:MyResource")
            number = 2
        elif name == "testResource2":
            self.assertEqual(ty, "test:index:MyResource")
            number = 3
        elif name == "testResource3":
            self.assertEqual(ty, "test:index:FinalResource")
            # The source program uses Output.apply to merge outputs from the above two resources.
            # The 5 is produced by adding 2 and 3 in the source program.
            self.assertEqual(_resource["number"], 5)
            number = _resource["number"]
        elif name == "testResource4":
            self.assertEqual(ty, "test:index:FinalResource")
            # The source program uses Output.apply to merge outputs from the above two resources.
            # The 5 is produced by adding 2 and 3 in the source program.
            self.assertEqual(_resource["number"], 5)
            number = _resource["number"]
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": {
                "number": number
            }
        }
