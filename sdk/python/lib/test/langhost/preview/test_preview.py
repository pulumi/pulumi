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


class PreviewTest(LanghostTest):
    """
    Test that tests that pulumi.runtime.is_dry_run actually returns True on previews and False on updates.
    """
    def test_preview(self):
        self.run_test(
            program=path.join(self.base_path(), "preview"),
            expected_resource_count=1)

    def register_resource(self, _ctx, dry_run, ty, name, resource,
                          _dependencies, _parent, _custom, _protect, _provider, _property_deps, _delete_before_replace,
                          _ignore_changes, _version):
        self.assertEqual(ty, "test:index:MyResource")
        self.assertEqual(name, "foo")
        if dry_run:
            self.assertDictEqual({
                "is_preview": True
            }, resource)
        else:
            self.assertDictEqual({
                "is_preview": False
            }, resource)
        return {
            "urn": self.make_urn(ty, name),
        }
