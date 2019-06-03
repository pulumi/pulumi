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


class TestOutputsFuture(LanghostTest):
    def test_outputs_future(self):
        self.run_test(
            program=path.join(self.base_path(), "outputs_future"),
            expected_resource_count=0)

    def invoke(self, _ctx, token, args, _provider, _version):
        self.assertEqual("test:index:MyFunction", token)
        self.assertDictEqual({
            "value": 41,
        }, args)

        return [], {
            "value": args["value"] + 1
        }

    def register_resource_outputs(self, _ctx, _dry_run, _urn, ty, _name, _resource, outputs):
        self.assertEqual(ty, "pulumi:pulumi:Stack")
        self.assertDictEqual({
            "value": 42
        }, outputs)
