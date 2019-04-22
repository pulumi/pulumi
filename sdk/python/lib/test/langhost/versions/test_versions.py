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


class TestVersions(LanghostTest):
    def test_versions(self):
        self.run_test(
            program=path.join(self.base_path(), "versions"),
            expected_resource_count=3)

    def register_resource(self, ctx, dry_run, ty, name, _resource,
                          _dependencies, _parent, _custom, _protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, version):
        if name == "testres":
            self.assertEqual(version, "0.19.1")
        elif name == "testres2":
            self.assertEqual(version, "0.19.2")
        elif name == "testres3":
            self.assertEqual(version, "")
        else:
            self.fail(f"unknown resource: {name}")
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": {}
        }

    def invoke(self, _ctx, token, args, _provider, version):
        if token == "test:index:doit":
            self.assertEqual("0.19.1", version)
        elif token == "test:index:doit_v2":
            self.assertEqual("0.19.2", version)
        elif token == "test:index:doit_no_version":
            self.assertEqual("", version)
        return [], args
