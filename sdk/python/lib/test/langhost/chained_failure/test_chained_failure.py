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


class ChainedFailureTest(LanghostTest):
    """
    Tests that the language host can tolerate "chained failures" - that is, a failure of an output to resolve when
    attempting to prepare a resource for registration.

    In this test, the program raises an exception in an apply, which causes the preparation of resourceB to fail. This
    test asserts that this does not cause a deadlock (as it previously did, pulumi/pulumi#2189) but instead terminates
    gracefully.
    """
    def test_chained_failure(self):
        self.run_test(
            program=path.join(self.base_path(), "chained_failure"),
            expected_error="Program exited with non-zero exit code: 1",
            expected_resource_count=1)

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if ty == "test:index:ResourceA":
            self.assertEqual(name, "resourceA")
            self.assertDictEqual(_resource, {"inprop": 777})
            return {
                "urn": self.make_urn(ty, name),
                "id": name,
                "object": {
                    "outprop": 200
                } 
            }

        if ty == "test:index:ResourceB":
            self.fail(f"we should never have gotten here! {_resource}")
        self.fail(f"unknown resource type: {ty}")
