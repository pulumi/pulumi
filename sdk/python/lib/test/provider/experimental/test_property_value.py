from pulumi.provider.experimental.property_value import PropertyValue, PropertyValueType
import collections.abc as abc
import pytest


def test_property_value_type():
    value = PropertyValue(2.0)
    assert value.type == PropertyValueType.NUMBER

    value = PropertyValue("hello")
    assert value.type == PropertyValueType.STRING

    value = PropertyValue(True)
    assert value.type == PropertyValueType.BOOL

    value = PropertyValue(None)
    assert value.type == PropertyValueType.NULL

    value = PropertyValue.computed()
    assert value.type == PropertyValueType.COMPUTED

    value = PropertyValue([PropertyValue(1.0), PropertyValue(2.0), PropertyValue(3.0)])
    assert value.type == PropertyValueType.ARRAY

    value = PropertyValue({"a": PropertyValue(1.0), "b": PropertyValue(2.0)})
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
    assert pbvalue.list_value.values[0].number_value == 2.0
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


def test_unsupported_value_type():
    """Test that unsupported value types raise TypeError."""

    # Test with an unsupported type (e.g., a custom class)
    class UnsupportedType:
        pass

    with pytest.raises(
        TypeError,
        match=r"Unsupported value type: UnsupportedType\. Expected one of:",
    ):
        PropertyValue(UnsupportedType())  # type: ignore

    # Test with int (only float is supported)
    with pytest.raises(
        TypeError,
        match=r"Unsupported value type: int\. Expected one of:",
    ):
        PropertyValue(42)  # type: ignore


def test_invalid_is_secret_type():
    """Test that is_secret must be a bool."""
    with pytest.raises(TypeError, match=r"is_secret must be a bool, got str\."):
        PropertyValue("test", is_secret="true")  # type: ignore

    with pytest.raises(TypeError, match=r"is_secret must be a bool, got int\."):
        PropertyValue("test", is_secret=1)  # type: ignore


def test_invalid_dependencies_type():
    """Test that dependencies must be an Iterable[str] or None."""
    # Test with non-iterable type
    with pytest.raises(
        TypeError, match=r"dependencies must be an Iterable\[str\] or None, got int\."
    ):
        PropertyValue("test", dependencies=123)  # type: ignore

    # Test with iterable containing non-string items
    with pytest.raises(
        TypeError, match=r"All dependencies must be strings, found int\."
    ):
        PropertyValue("test", dependencies=["valid", 123, "also-valid"])  # type: ignore


def test_invalid_sequence_items():
    """Test that Sequence items must be PropertyValue instances."""
    # Test with sequence containing non-PropertyValue items
    with pytest.raises(
        TypeError,
        match=r"Sequence items must be PropertyValue instances, found str at index 1\.",
    ):
        PropertyValue([PropertyValue(1.0), "invalid", PropertyValue(3.0)])  # type: ignore

    # Test with sequence containing int
    with pytest.raises(
        TypeError,
        match=r"Sequence items must be PropertyValue instances, found int at index 0\.",
    ):
        PropertyValue([1, 2, 3])  # type: ignore


def test_invalid_mapping_keys():
    """Test that Mapping keys must be strings."""
    # Test with non-string keys
    with pytest.raises(TypeError, match=r"Mapping keys must be strings, found int\."):
        PropertyValue({1: PropertyValue("value")})  # type: ignore

    with pytest.raises(TypeError, match=r"Mapping keys must be strings, found bool\."):
        PropertyValue({True: PropertyValue("value")})  # type: ignore


def test_invalid_mapping_values():
    """Test that Mapping values must be PropertyValue instances."""
    # Test with non-PropertyValue values
    with pytest.raises(
        TypeError,
        match=r"Mapping values must be PropertyValue instances, found str for key 'a'\.",
    ):
        PropertyValue({"a": "invalid"})  # type: ignore

    with pytest.raises(
        TypeError,
        match=r"Mapping values must be PropertyValue instances, found int for key 'count'\.",
    ):
        PropertyValue({"name": PropertyValue("test"), "count": 42})  # type: ignore


def test_valid_value_types():
    """Test that all valid value types are accepted."""
    from pulumi.provider.experimental.property_value import (
        ResourceReference,
        Computed,
    )
    import pulumi

    # Test all supported types
    PropertyValue(None)  # None
    PropertyValue(True)  # bool
    PropertyValue(3.14)  # float
    PropertyValue("test")  # str
    PropertyValue(pulumi.FileAsset("path/to/file"))  # Asset
    PropertyValue(pulumi.FileArchive("path/to/archive"))  # Archive
    PropertyValue([PropertyValue(1.0)])  # Sequence
    PropertyValue({"key": PropertyValue(1.0)})  # Mapping
    PropertyValue(ResourceReference(urn="urn:test"))  # ResourceReference
    PropertyValue(Computed())  # Computed


def test_valid_dependencies():
    """Test that valid dependencies are accepted."""
    # Test with list of strings
    PropertyValue("test", dependencies=["dep1", "dep2"])

    # Test with set of strings
    PropertyValue("test", dependencies={"dep1", "dep2"})

    # Test with tuple of strings
    PropertyValue("test", dependencies=("dep1", "dep2"))

    # Test with None (default)
    PropertyValue("test", dependencies=None)
