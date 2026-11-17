# Copyright 2026, Pulumi Corporation.
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

# mypy: warn_unused_ignores = True

from typing_extensions import assert_type

from pulumi import Output
from pulumi.runtime import test


@test
def check_value(value: str, *, count: int = 1) -> str:
    return value * count


@test
def check_output(value: str) -> Output[str]:
    return Output.from_input(value)


@test
async def check_async(value: str) -> str:
    return value


def check_signatures() -> None:
    assert_type(check_value("value"), None)
    assert_type(check_value("value", count=2), None)
    assert_type(check_output("value"), None)
    assert_type(check_async("value"), None)

    # These calls deliberately pass invalid arguments, e.g. 123 instead of a str.
    # Each "type: ignore" suppresses the expected error. With warn_unused_ignores
    # enabled above, mypy also checks that there was an error to suppress.
    # If @test loses the argument types, these bad calls no longer produce errors,
    # so mypy reports the ignores as unused and this test fails.
    check_value(123)  # type: ignore[arg-type]
    check_value("value", 2)  # type: ignore[call-arg]
    check_value("value", count="two")  # type: ignore[arg-type]
