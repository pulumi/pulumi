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
import unittest
from ..util import LanghostTest

# This tests the creation of a resource that contains a very large string.
# In particular we are testing sending large message sizes (>4mb) over an RPC call.

long_string = "a" * 1024 * 1024 * 5


class LargeResourceTest(LanghostTest):
    def test_large_resource(self):
        self.run_test(
            program=path.join(self.base_path(), "large_resource"),
            expected_resource_count=1)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        self.assertEqual(ty, "test:index:MyLargeStringResource")
        self.assertEqual(name, "testResource1")

        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": {
                "largeStringProp": long_string,
            }
        }
