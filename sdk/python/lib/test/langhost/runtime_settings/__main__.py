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
import pulumi


config = pulumi.Config("test")
assert pulumi.get_project() == "myproject"
assert pulumi.get_stack() == "mystack"
assert config.get("known") == "knownkey"
assert config.get("unknown") is None
assert config.get_bool('lowercase_true')
assert config.get_bool('uppercase_true')
assert not config.get_bool('lowercase_false')
assert not config.get_bool('uppercase_false')
try:
    config.get_bool('not_a_bool')
except pulumi.ConfigTypeError as exn:
    assert exn.key == 'test:not_a_bool'
    assert exn.expect_type == 'bool'
