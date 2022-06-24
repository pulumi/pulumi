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


class InheritanceTranslationTest(LanghostTest):
    def test_inheritance_translation(self):
        self.run_test(
            program=path.join(self.base_path(), "inheritance_translation"),
            expected_resource_count=4)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        self.assertEqual("test:index:MyResource", ty)
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": {"someValue": "hello", "anotherValue": "world"},
        }

    def register_resource_outputs(self, _ctx, _dry_run, _urn, ty, _name, _resource, outputs):
        self.assertEqual("pulumi:pulumi:Stack", ty)
        self.assertEqual({
            "r1.some_value": "hello",
            "r1.another_value": "world",
            "r1.combined_values": "hello world",
            "r2.some_value": "hello",
            "r2.another_value": "world",
            "r2.combined_values": "hello world",
            "r2.new_value": "hello world!",
            "r3.some_value": "hello",
            "r3.another_value": "world",
            "r3.combined_values": "hello world",
            "r4.some_value": "hello",
            "r4.another_value": "world",
            "r4.combined_values": "hello world",
            "r4.new_value": "hello world!",
        }, outputs)
