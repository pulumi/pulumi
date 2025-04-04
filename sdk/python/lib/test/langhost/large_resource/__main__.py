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
import functools
from pulumi import CustomResource, Output

long_string = "a" * 1024 * 1024 * 5


class MyResource(CustomResource):
    largeStringProp: Output[str]

    def __init__(self, name):
        CustomResource.__init__(
            self,
            "test:index:MyLargeStringResource",
            name,
            props={
                "largeStringProp": long_string,
            },
        )


def assert_eq(lhs, rhs):
    assert lhs == rhs


res = MyResource("testResource1")
res.largeStringProp.apply(functools.partial(assert_eq, long_string))
