# Copyright 2016-2023, Pulumi Corporation.
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

from pulumi import Output
from pulumi.runtime import invoke, invoke_async


def assert_eq(l, r):
    assert l == r


ret = invoke("test:index:MyFunction", {})
assert ret.value == {}, "Expected the return value of the invoke to be an empty dict"

ret2 = Output.from_input(invoke_async("test:index:MyFunction", {})).apply(
    lambda v: assert_eq(v, {})
)
