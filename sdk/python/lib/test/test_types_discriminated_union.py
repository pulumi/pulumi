# Copyright 2016, Pulumi Corporation.
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

import asyncio
import unittest
from typing import Literal, Optional, Union

import pulumi
from pulumi import _types
from pulumi.runtime import rpc


@pulumi.input_type
class CatArgs:
    kind: pulumi.Input[Literal["cat"]] = pulumi.property("kind")
    lives_left: pulumi.Input[int] = pulumi.property("livesLeft")


@pulumi.input_type
class DogArgs:
    kind: pulumi.Input[Literal["dog"]] = pulumi.property("kind")
    good_boy: pulumi.Input[bool] = pulumi.property("goodBoy")


# Output types are declared the way codegen emits them: a `dict` subclass whose properties are
# reached through getters.
@pulumi.output_type
class Cat(dict):
    def __init__(__self__, *, kind: Literal["cat"], lives_left: int):
        pulumi.set(__self__, "kind", kind)
        pulumi.set(__self__, "lives_left", lives_left)

    @property
    @pulumi.getter
    def kind(self) -> Literal["cat"]:
        return pulumi.get(self, "kind")

    @property
    @pulumi.getter(name="livesLeft")
    def lives_left(self) -> int:
        return pulumi.get(self, "lives_left")


@pulumi.output_type
class Dog(dict):
    def __init__(__self__, *, kind: Literal["dog"], good_boy: bool):
        pulumi.set(__self__, "kind", kind)
        pulumi.set(__self__, "good_boy", good_boy)

    @property
    @pulumi.getter
    def kind(self) -> Literal["dog"]:
        return pulumi.get(self, "kind")

    @property
    @pulumi.getter(name="goodBoy")
    def good_boy(self) -> bool:
        return pulumi.get(self, "good_boy")


# Two cases with identical shapes, distinguished only by their tag.
@pulumi.output_type
class Metric(dict):
    kind: Literal["metric"] = pulumi.property("kind")
    amount: float = pulumi.property("amount")


@pulumi.output_type
class Imperial(dict):
    kind: Literal["imperial"] = pulumi.property("kind")
    amount: float = pulumi.property("amount")


# Members that pin more than one property to a constant. Codegen emits a Literal for every schema
# constant, not just for a discriminator, so this is the common shape.
@pulumi.output_type
class ConfigMap(dict):
    kind: Literal["ConfigMap"] = pulumi.property("kind")
    api_version: Literal["v1"] = pulumi.property("apiVersion")
    data: str = pulumi.property("data")


@pulumi.output_type
class Secret(dict):
    kind: Literal["Secret"] = pulumi.property("kind")
    api_version: Literal["v1"] = pulumi.property("apiVersion")
    data: str = pulumi.property("data")


# Constants that are not strings.
@pulumi.output_type
class VersionOne(dict):
    version: Literal[1] = pulumi.property("version")
    legacy: Literal[True] = pulumi.property("legacy")


@pulumi.output_type
class VersionTwo(dict):
    version: Literal[2] = pulumi.property("version")
    legacy: Literal[False] = pulumi.property("legacy")


# Two members pinning the same property to the same value cannot be told apart.
@pulumi.output_type
class SameConstantA(dict):
    kind: Literal["same"] = pulumi.property("kind")


@pulumi.output_type
class SameConstantB(dict):
    kind: Literal["same"] = pulumi.property("kind")


# The constant's Pulumi and Python names differ, so each entry point reads only its own.
@pulumi.output_type
class TheVersionOne(dict):
    the_version: Literal[1] = pulumi.property("theVersion")


@pulumi.output_type
class TheVersionTwo(dict):
    the_version: Literal[2] = pulumi.property("theVersion")


@pulumi.output_type
class Untagged(dict):
    name: str = pulumi.property("name")


InputUnion = pulumi.Input[Optional[Union[CatArgs, DogArgs]]]
OutputUnion = Optional[Union[Cat, Dog]]


def serialize(value, typ):
    return asyncio.new_event_loop().run_until_complete(
        rpc.serialize_property(value, [], "pet", None, None, typ)
    )


def translate(output, typ):
    return rpc.translate_output_properties(output, lambda k: k, typ, True)


class ReduceTests(unittest.TestCase):
    def test_reduces_to_the_member_the_constants_select(self):
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                InputUnion, {"kind": "cat", "livesLeft": 9}
            ),
            CatArgs,
        )
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                OutputUnion, {"kind": "dog", "goodBoy": True}
            ),
            Dog,
        )

    def test_reduces_by_python_name(self):
        # A user writing a dict by hand uses the Python name of the property.
        self.assertIs(
            _types.reduce_discriminated_union_by_python_name(
                OutputUnion, {"kind": "cat", "lives_left": 9}
            ),
            Cat,
        )

    def test_members_sharing_a_shape_stay_distinct(self):
        union = Union[Metric, Imperial]
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                union, {"kind": "metric", "amount": 1.0}
            ),
            Metric,
        )
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                union, {"kind": "imperial", "amount": 1.0}
            ),
            Imperial,
        )

    def test_members_with_several_constants(self):
        # Codegen emits a Literal for every schema constant, so a member may pin more than one
        # property. The whole set is matched, and the property that actually differs decides.
        union = Union[ConfigMap, Secret]
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                union, {"kind": "ConfigMap", "apiVersion": "v1", "data": "d"}
            ),
            ConfigMap,
        )
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                union, {"kind": "Secret", "apiVersion": "v1", "data": "d"}
            ),
            Secret,
        )

    def test_a_shared_constant_alone_does_not_decide(self):
        # `apiVersion` agrees with both members, so it must not select one on its own.
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(
                Union[ConfigMap, Secret], {"apiVersion": "v1"}
            )
        )

    def test_non_string_constants(self):
        union = Union[VersionOne, VersionTwo]
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                union, {"version": 1, "legacy": True}
            ),
            VersionOne,
        )
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(
                union, {"version": 2, "legacy": False}
            ),
            VersionTwo,
        )

    def test_bool_is_not_confused_with_int(self):
        # `True == 1` in Python, so a bool constant must not match an integer value.
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(
                Union[VersionOne, VersionTwo], {"version": 1, "legacy": 1}
            )
        )

    def test_absent_constants_select_nothing(self):
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(
                OutputUnion, {"goodBoy": True}
            )
        )

    def test_unknown_constant_selects_nothing(self):
        # An SDK older than the provider sees a value it does not know. It must not raise.
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(
                OutputUnion, {"kind": "fish"}
            )
        )

    def test_members_that_cannot_be_told_apart(self):
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(
                Union[SameConstantA, SameConstantB], {"kind": "same"}
            )
        )

    def test_member_without_constants_is_not_a_union(self):
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(
                Union[Cat, Untagged], {"kind": "cat"}
            )
        )

    def test_non_union_and_single_member(self):
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(Cat, {"kind": "cat"})
        )
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(
                Optional[Cat], {"kind": "cat"}
            )
        )


class DirectionTests(unittest.TestCase):
    # Each entry point reads the value under one naming convention only.
    def test_pulumi_names_resolve_on_the_pulumi_side(self):
        union = Union[TheVersionOne, TheVersionTwo]
        self.assertIs(
            _types.reduce_discriminated_union_by_pulumi_name(union, {"theVersion": 2}),
            TheVersionTwo,
        )
        self.assertIsNone(
            _types.reduce_discriminated_union_by_pulumi_name(union, {"the_version": 2})
        )

    def test_python_names_resolve_on_the_python_side(self):
        union = Union[TheVersionOne, TheVersionTwo]
        self.assertIs(
            _types.reduce_discriminated_union_by_python_name(union, {"the_version": 2}),
            TheVersionTwo,
        )
        self.assertIsNone(
            _types.reduce_discriminated_union_by_python_name(union, {"theVersion": 2})
        )


class IsDiscriminatedUnionTests(unittest.TestCase):
    def test_recognises_constrained_unions(self):
        self.assertTrue(_types.is_discriminated_union(InputUnion))
        self.assertTrue(_types.is_discriminated_union(OutputUnion))
        self.assertTrue(_types.is_discriminated_union(Union[ConfigMap, Secret]))

    def test_rejects_everything_else(self):
        self.assertFalse(_types.is_discriminated_union(Union[Cat, Untagged]))
        self.assertFalse(_types.is_discriminated_union(Cat))
        self.assertFalse(_types.is_discriminated_union(Optional[Cat]))


class SerializeInputTests(unittest.TestCase):
    def test_generated_class_is_serialized_with_pulumi_names(self):
        self.assertEqual(
            serialize(CatArgs(kind="cat", lives_left=9), InputUnion),
            {"kind": "cat", "livesLeft": 9},
        )

    def test_plain_dict_is_serialized_with_pulumi_names(self):
        # The case a user hits when they skip the generated class and write a dict.
        self.assertEqual(
            serialize({"kind": "cat", "lives_left": 9}, InputUnion),
            {"kind": "cat", "livesLeft": 9},
        )

    def test_plain_dict_selects_the_right_member(self):
        self.assertEqual(
            serialize({"kind": "dog", "good_boy": True}, InputUnion),
            {"kind": "dog", "goodBoy": True},
        )

    def test_output_value_round_trips_into_an_input(self):
        # One resource's output assigned straight into another resource's input, which is what
        # the l2-discriminated-union-many conformance test does.
        value = translate({"kind": "dog", "goodBoy": True}, OutputUnion)
        self.assertEqual(serialize(value, InputUnion), {"kind": "dog", "goodBoy": True})

    def test_unknown_constant_is_left_alone(self):
        value = {"kind": "fish", "lives_left": 1}
        self.assertEqual(serialize(value, InputUnion), value)


class TranslateOutputTests(unittest.TestCase):
    def test_translates_into_the_selected_member(self):
        result = translate({"kind": "cat", "livesLeft": 9}, OutputUnion)
        self.assertIsInstance(result, Cat)
        self.assertEqual(result.lives_left, 9)

    def test_translates_the_other_member(self):
        result = translate({"kind": "dog", "goodBoy": True}, OutputUnion)
        self.assertIsInstance(result, Dog)
        self.assertEqual(result.good_boy, True)

    def test_members_sharing_a_shape_are_told_apart(self):
        union = Union[Metric, Imperial]
        self.assertIsInstance(
            translate({"kind": "metric", "amount": 1.0}, union), Metric
        )
        self.assertIsInstance(
            translate({"kind": "imperial", "amount": 1.0}, union), Imperial
        )

    def test_members_with_several_constants(self):
        union = Union[ConfigMap, Secret]
        result = translate({"kind": "Secret", "apiVersion": "v1", "data": "d"}, union)
        self.assertIsInstance(result, Secret)
        self.assertEqual(result.api_version, "v1")

    def test_absent_constants_leave_the_value_untyped(self):
        # Must not raise. A union the runtime cannot reduce degrades to an untyped dict.
        self.assertEqual(translate({"livesLeft": 9}, OutputUnion), {"livesLeft": 9})

    def test_unknown_constant_leaves_the_value_untyped(self):
        result = translate({"kind": "fish", "fins": 2}, OutputUnion)
        self.assertEqual(result, {"kind": "fish", "fins": 2})


if __name__ == "__main__":
    unittest.main()
