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
import unittest
from ..util import LanghostTest

from pulumi.runtime.rpc import Unknown

class ResourceThensTest(LanghostTest):
    def test_resource_thens(self):
        self.run_test(
            program=path.join(self.base_path(), "resource_thens"),
            expected_resource_count=2)

    def register_resource(self, _ctx, dry_run, ty, name, resource,
                          _dependencies):
        id_ = None
        props = resource
        props["stable"] = "yeah"
        if ty == "test:index:MyResource":
            self.assertEqual(name, "first")
            self.assertEqual(resource["foo"], "bar")
            if not dry_run:
                id_ = name
                props["outprop"] = "output yeah"
        elif ty == "test:index:OtherResource":
            self.assertEqual(name, "second")
            if dry_run:
                self.assertIsInstance(resource["inprop"], Unknown)
            else:
                self.assertEqual(resource["inprop"], "output yeah")

            if not dry_run:
                id_ = name

        return {
            "urn": self.make_urn(ty, name),
            "id": id_,
            "object": props
        }
