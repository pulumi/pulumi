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


class RuntimeSettingsTest(LanghostTest):
    def test_runtime_settings(self):
        self.run_test(program=path.join(self.base_path(), "runtime_settings"),
                      organization="myorg",
                      project="myproject",
                      stack="mystack",
                      config={
                          "test:known": "knownkey",
                          "test:lowercase_true": "true",
                          "test:uppercase_true": "True",
                          "test:lowercase_false": "false",
                          "test:uppercase_false": "False",
                          "test:not_a_bool": "DBBool",
                      },
                      expected_resource_count=0)
