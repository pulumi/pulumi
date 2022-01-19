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


class TestInvoke(LanghostTest):
    def test_invoke_success(self):
        self.run_test(
            program=path.join(self.base_path(), "invoke_types"),
            expected_resource_count=2)

    def invoke(self, _ctx, token, args, provider, _version):
        def result(expected_first_value: str, expected_second_value: float):
            self.assertDictEqual({
                "firstValue": expected_first_value,
                "secondValue": expected_second_value,
            }, args)
            return ([], {
                "nested": {
                    "firstValue": args["firstValue"] * 2,
                    "secondValue": args["secondValue"] + 1,
                },
            })

        if token == "test:index:MyFunction":
            return result("hello", 42)
        elif token == "test:index:MyOtherFunction":
            return result("world", 100)
        else:
            self.fail(f"unexpected token {token}")

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        if name == "resourceA":
            self.assertEqual({
                "first_value": "hellohello",
                "second_value": 43,
            }, _resource)
        elif name == "resourceB":
            self.assertEqual({
                "first_value": "worldworld",
                "second_value": 101,
            }, _resource)
        else:
            self.fail(f"unknown resource: {name}")

        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": _resource,
        }
