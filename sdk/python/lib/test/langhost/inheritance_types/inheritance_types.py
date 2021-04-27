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


class InheritanceTypesTest(LanghostTest):
    def test_inheritance_types(self):
        self.run_test(
            program=path.join(self.base_path(), "inheritance_types"),
            expected_resource_count=1)

    def register_resource(self, ctx, dry_run, ty, name, _resource,
                          _dependencies, _parent, _custom, _protect, _provider, _property_deps, _delete_before_replace,
                          _ignore_changes, _version):
        self.assertEqual("test:index:MyResource", ty)
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": {"foo": {"bar": "hello", "baz": "world"}},
        }

    def register_resource_outputs(self, _ctx, _dry_run, _urn, ty, _name, _resource, outputs):
        self.assertEqual("pulumi:pulumi:Stack", ty)
        self.assertEqual({"combined_values": "hello world"}, outputs)
