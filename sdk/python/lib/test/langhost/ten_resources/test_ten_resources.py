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


class TenResourcesTest(LanghostTest):
    def setUp(self):
        self.seen = {}

    def test_ten_resources(self):
        self.run_test(
            program=path.join(self.base_path(), "ten_resources"),
            expected_resource_count=10)

    def register_resource(self, ctx, dry_run, ty, name, _resource,
                          _dependencies):
        self.assertEqual("test:index:MyResource", ty)
        if not dry_run:
            self.assertIsNone(
                self.seen.get(name),
                "Got multiple resources with the same name: " + name)
            self.seen[name] = True
        return {"urn": self.make_urn(ty, name)}
