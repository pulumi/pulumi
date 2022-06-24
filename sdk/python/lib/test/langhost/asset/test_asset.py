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
from pulumi.asset import FileAsset, StringAsset, RemoteAsset
from ..util import LanghostTest


class AssetTest(LanghostTest):
    def test_asset(self):
        self.run_test(
            program=path.join(self.base_path(), "asset"),
            expected_resource_count=4)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        self.assertEqual(ty, "test:index:MyResource")
        if name == "file":
            self.assertIsInstance(_resource["asset"], FileAsset)
            self.assertEqual(path.normpath(_resource["asset"].path), "testfile.txt")
        elif name == "string":
            self.assertIsInstance(_resource["asset"], StringAsset)
            self.assertEqual(_resource["asset"].text, "its a string")
        elif name == "remote":
            self.assertIsInstance(_resource["asset"], RemoteAsset)
            self.assertEqual(_resource["asset"].uri, "https://pulumi.com")
        else:
            self.fail("unexpected resource name: " + name)
        return {
            "urn": self.make_urn(ty, name),
        }
