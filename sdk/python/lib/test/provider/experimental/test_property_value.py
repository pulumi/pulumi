import pulumi
from typing import Optional, TypedDict
from pulumi.provider.experimental.property_value import PropertyValue
import collections.abc as abc


def test_property_value_list():
    value = PropertyValue(
        [
            PropertyValue(2.0),
            PropertyValue("hello"),
        ]
    )

    pbvalue = PropertyValue.marshal(value)

    assert pbvalue.list_value is not None
    assert len(pbvalue.list_value.values) == 2
    assert pbvalue.list_value.values[0].number_value == 2
    assert pbvalue.list_value.values[1].string_value == "hello"

    value = PropertyValue.unmarshal(pbvalue)

    assert isinstance(value.value, abc.Sequence)
    assert len(value.value) == 2
    assert value.value[0] == PropertyValue(2.0)
    assert value.value[1] == PropertyValue("hello")


def test_property_value_struct():
    value = PropertyValue(
        {
            "a": PropertyValue(True),
            "b": PropertyValue(None),
        }
    )

    pbvalue = PropertyValue.marshal(value)

    assert pbvalue.struct_value is not None
    assert len(pbvalue.struct_value.fields) == 2
    assert pbvalue.struct_value.fields["a"].bool_value == True
    assert pbvalue.struct_value.fields["b"].WhichOneof("kind") == "null_value"

    value = PropertyValue.unmarshal(pbvalue)

    assert isinstance(value.value, abc.Mapping)
    assert len(value.value) == 2
    assert value.value["a"] == PropertyValue(True)
    assert value.value["b"] == PropertyValue(None)


def test_property_value_map():
    value = {
        "a": PropertyValue(True),
        "b": PropertyValue(None),
    }

    pbvalue = PropertyValue.marshal_map(value)
    result = PropertyValue.unmarshal_map(pbvalue)
    assert value == result


async def test_nesting():
    class RecursiveTypeA(TypedDict):
        b: Optional[pulumi.Input["RecursiveTypeB"]]

    class RecursiveTypeB(TypedDict):
        a: Optional[pulumi.Input[RecursiveTypeA]]

    class Args(TypedDict):
        rec: pulumi.Input[RecursiveTypeA]

    value = {
        "rec": PropertyValue(
            {"a": PropertyValue({"b": PropertyValue({"a": PropertyValue(None)})})}
        )
    }
    obj = PropertyValue.deserialize_map(value)

    assert obj == {"rec": {"a": {"b": {}}}}

    pbvalue = await PropertyValue.serialize_map(obj, None, None)
    assert value == pbvalue
