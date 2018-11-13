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


class TestInvoke(LanghostTest):
    def test_invoke_success(self):
        self.run_test(
            program=path.join(self.base_path(), "invoke"),
            expected_resource_count=1)

    def invoke(self, _ctx, token, args):
        self.assertEqual("test:index:MyFunction", token)
        self.assertDictEqual({
            "value": 41,
        }, args)

        return [], {
            "value": args["value"] + 1
        }

    def register_resource(self, _ctx, _dry_run, ty, name, resource, _deps):
        self.assertEqual("test:index:MyResource", ty)
        self.assertEqual("resourceA", name)
        self.assertEqual(resource["value"], 42)

        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": resource,
        }


class TestInvokeWithFailures(LanghostTest):
    def test_invoke_failure(self):
        self.run_test(
            program=path.join(self.base_path(), "invoke"),
            expected_resource_count=0,
            expected_error="Program exited with non-zero exit code: 1")

    def invoke(self, _ctx, token, args):
        self.assertEqual("test:index:MyFunction", token)
        self.assertDictEqual({
            "value": 41,
        }, args)

        return [{"property": "value", "reason": "the invoke failed"}], {}

