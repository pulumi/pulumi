from pulumi.provider.experimental.property_value import PropertyValue, PropertyValueType
import collections.abc as abc


def test_property_value_type():
    value = PropertyValue(2.0)
    assert value.type == PropertyValueType.NUMBER

    value = PropertyValue(42)
    assert value.type == PropertyValueType.NUMBER

    value = PropertyValue("hello")
    assert value.type == PropertyValueType.STRING

    value = PropertyValue(True)
    assert value.type == PropertyValueType.BOOL

    value = PropertyValue(None)
    assert value.type == PropertyValueType.NULL

    value = PropertyValue.computed()
    assert value.type == PropertyValueType.COMPUTED

    value = PropertyValue([1, 2, 3])
    assert value.type == PropertyValueType.ARRAY

    value = PropertyValue({"a": 1, "b": 2})
    assert value.type == PropertyValueType.MAP


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


def test_nesting():
    value = {
        "rec": PropertyValue(
            {"a": PropertyValue({"b": PropertyValue({"a": PropertyValue(None)})})}
        )
    }
    pbvalue = PropertyValue.marshal_map(value)
    result = PropertyValue.unmarshal_map(pbvalue)
    assert value == result


def test_marshal_int_as_number():
    value = PropertyValue(42)
    pbvalue = value.marshal()

    assert pbvalue.number_value == 42
