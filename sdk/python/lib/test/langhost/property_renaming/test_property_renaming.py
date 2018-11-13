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


class PropertyRenamingTest(LanghostTest):
    """
    Tests that Pulumi resources can override translate_input_property and translate_output_property
    in order to control the naming of their own properties.
    """
    def test_property_renaming(self):
        self.run_test(
            program=path.join(self.base_path(), "property_renaming"),
            expected_resource_count=1)

    def register_resource(self, _ctx, _dry_run, ty, name, res, _deps):
        # Test:
        #  1. Everything that we receive from the running program is in camel-case. The engine never sees
        # the pre-translated names of the input properties.
        #  2. We return properties back to the running program in camel case. It's the responsibility of the SDK
        # to translate them back to snake case.
        self.assertEqual("test:index:TranslatedResource", ty)
        self.assertEqual("res", name)
        self.assertIn("engineProp", res)
        self.assertEqual("some string", res["engineProp"])
        self.assertIn("recursiveProp", res)
        self.assertDictEqual({
            "recursiveKey": "value"
        }, res["recursiveProp"])
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": {
                "engineProp": "some string",
                "engineOutputProp": "some output string",
                "recursiveProp": {
                    "recursiveKey": "value",
                    "recursiveOutput": "some other output"
                }
            }
        }

    def register_resource_outputs(self, _ctx, _dry_run, _urn, ty, _name, _resource, outputs):
        self.assertEqual(ty, "pulumi:pulumi:Stack")
        # Despite operating entirely in terms of camelCase above in register resource, the outputs
        # received from the program are all in snake case.
        self.assertDictEqual({
            "transformed_prop": "some string",
            "engine_output_prop": "some output string",
            "recursive_prop": {
                "recursive_key": "value",
                "recursive_output": "some other output"
            }
        }, outputs)
