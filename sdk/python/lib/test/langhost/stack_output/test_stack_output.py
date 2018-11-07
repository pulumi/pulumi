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


class StackOutputTest(LanghostTest):
    """
    Test that tests Pulumi's ability to register resource outputs.
    """
    def test_ten_resources(self):
        self.run_test(
            program=path.join(self.base_path(), "stack_output"),
            expected_resource_count=0)

    def register_resource_outputs(self, _ctx, _dry_run, _urn, ty, _name, _resource, outputs):
        self.assertEqual(ty, "pulumi:stack:Stack")
        self.assertDictEqual({
            "the-coolest": "pulumi"
        }, outputs)
