# Copyright 2016-2019, Pulumi Corporation.
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


class ReadTest(LanghostTest):
    def test_read(self):
        self.run_test(
            program=path.join(self.base_path(), "read"),
            expected_resource_count=1)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource,
                          _dependencies, _parent, _custom, _protect, _provider, _property_deps, _delete_before_replace,
                          _ignore_changes, _version):
        self.assertEqual(ty, "test:index:MyResource")
        self.assertEqual(name, "foo2")
        return {
            "urn": self.make_urn(ty, name),
        }

    def read_resource(self, ctx, ty, name, id, parent, state, dependencies, provider, version):
        if name == "foo":
            self.assertDictEqual(state, {
                "a": "bar",
                "b": ["c", 4, "d"],
                "c": {
                    "nest": "baz"
                }
            })
            self.assertEqual(ty, "test:read:resource")
            self.assertEqual(id, "myresourceid")
            self.assertEqual(version, "0.17.9")
        elif name == "foo-with-parent":
            self.assertDictEqual(state, {
                "state": "foo",
            })
            self.assertEqual(ty, "test:read:resource")
            self.assertEqual(id, "myresourceid2")
            self.assertEqual(parent, self.make_urn("test:index:MyResource", "foo2"))
            self.assertEqual(version, "0.17.9")
        return {
            "urn": self.make_urn(ty, name),
            "properties": state,
        }
