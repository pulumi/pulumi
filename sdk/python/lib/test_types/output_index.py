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

from collections.abc import Mapping, Sequence
from typing import Any

from typing_extensions import assert_type

from pulumi import Output


class Index:
    def __index__(self) -> int:
        return 0


class CustomIndexable:
    def __getitem__(self, key: str) -> float:
        return float(key)


def check_lists(names: Output[list[str]]) -> None:
    assert_type(names[0], Output[str])
    assert_type(names[-1], Output[str])
    assert_type(names[Index()], Output[str])
    assert_type(names[:2], Output[list[str]])
    assert_type(names[::2], Output[list[str]])
    assert_type(names[0].apply(str.upper), Output[str])

    # These invalid keys must produce errors. If the annotations stop rejecting
    # them, warn_unused_ignores (enabled above) makes mypy report the ignores
    # as unused.
    names["first"]  # type: ignore[call-overload]
    names[1.5]  # type: ignore[call-overload]


def check_dicts(
    labels: Output[dict[str, str]], records: Output[dict[int, list[str]]]
) -> None:
    assert_type(labels["env"], Output[str])
    assert_type(records[1], Output[list[str]])
    assert_type(records[1][0], Output[str])

    # These invalid keys must produce errors. If the annotations stop rejecting
    # them, warn_unused_ignores (enabled above) makes mypy report the ignores
    # as unused.
    labels[0]  # type: ignore[index]
    records["first"]  # type: ignore[index]


def check_other_indexables(
    mapping: Output[Mapping[str, int]],
    sequence: Output[Sequence[str]],
    values: Output[tuple[int, ...]],
    text: Output[str],
    data: Output[bytes],
    custom: Output[CustomIndexable],
    dynamic: Output[Any],
) -> None:
    assert_type(mapping["count"], Output[int])
    assert_type(sequence[0], Output[str])
    assert_type(sequence[:2], Output[Sequence[str]])
    assert_type(values[0], Output[int])
    assert_type(values[:2], Output[tuple[int, ...]])
    assert_type(text[0], Output[str])
    assert_type(text[:2], Output[str])
    assert_type(data[0], Output[int])
    assert_type(data[:2], Output[bytes])
    assert_type(custom["3.14"], Output[float])
    assert_type(dynamic["anything"], Output[Any])

    # These invalid keys must produce errors. If the annotations stop rejecting
    # them, warn_unused_ignores (enabled above) makes mypy report the ignores
    # as unused.
    custom[0]  # type: ignore[index]
