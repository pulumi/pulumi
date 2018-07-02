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
from pulumi.runtime.rpc import Unknown
from ..util import LanghostTest


class PreviewTest(LanghostTest):
    """
    This test tries to re-create a common setup for doing previews
    in Python.

    It is extremely common to use the "id" fields of resources to
    associate one resource to another. This test does this so that
    we are absolutely sure that it works for previews, otherwise
    previews in Python are pretty useless.
    """
    def test_preview(self):
        self.run_test(
            program=path.join(self.base_path(), "preview"),
            expected_resource_count=2)

    def register_resource(self, _ctx, dry_run, ty, name, resource,
                          _dependencies):
        id_ = None
        props = resource
        props["stable"] = "cool stable"
        if ty == "test:index:Bucket":
            self.assertEqual(name, "mybucket")
            if not dry_run:
                id_ = "mybucketid"
        elif ty == "test:index:BucketObject":
            self.assertEqual(name, "mybucketobject")
            if dry_run:
                self.assertIsInstance(resource["bucket"], Unknown)
            else:
                self.assertEqual(resource["bucket"], "mybucketid")

            if not dry_run:
                id_ = name

        return {
            "urn": self.make_urn(ty, name),
            "id": id_,
            "object": props
        }
