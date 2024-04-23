# Copyright 2016-2020, Pulumi Corporation.
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
from pulumi.runtime import invoke

import outputs


def my_function(first_value: str, second_value: float) -> outputs.MyFunctionResult:
    return invoke(
        "test:index:MyFunction",
        props={"firstValue": first_value, "secondValue": second_value},
        typ=outputs.MyFunctionResult,
    ).value


def my_other_function(
    first_value: str, second_value: float
) -> outputs.MyOtherFunctionResult:
    return invoke(
        "test:index:MyOtherFunction",
        props={"firstValue": first_value, "secondValue": second_value},
        typ=outputs.MyOtherFunctionResult,
    ).value


def assert_eq(l, r):
    assert l == r


class MyResource(pulumi.CustomResource):
    first_value: pulumi.Output[str]
    second_value: pulumi.Output[float]

    def __init__(self, name: str, first_value: str, second_value: float):
        super().__init__(
            "test:index:MyResource",
            name,
            {
                "first_value": first_value,
                "second_value": second_value,
            },
        )


result = my_function("hello", 42)
res = MyResource("resourceA", result.nested.first_value, result.nested.second_value)
res.first_value.apply(lambda v: assert_eq(v, "hellohello"))
res.second_value.apply(lambda v: assert_eq(v, 43))

result = my_other_function("world", 100)
res2 = MyResource("resourceB", result.nested.first_value, result.nested.second_value)
res2.first_value.apply(lambda v: assert_eq(v, "worldworld"))
res2.second_value.apply(lambda v: assert_eq(v, 101))
