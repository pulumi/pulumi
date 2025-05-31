# Copyright 2025, Pulumi Corporation.
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

from pulumi.resource import Resource, DependencyResource
from typing import (
    Any,
    Dict,
    cast,
    Sequence,
    Mapping,
)

import pulumi

from .property_value import PropertyValue, Computed, ResourceReference


class PropertyValueSerializer:
    """
    PropertyValueSerializer is a class that serializes back forth between PropertyValues and python values.
    """

    def deserialize(self, value: PropertyValue) -> Any:
        """
        Deserializes the PropertyValue into a Python value.

        :return: The deserialized Python value.
        """

        result: Any = None

        # If the value is Computed we need to return an unknown Output
        if isinstance(value, Computed):

            async def lift(v):
                return v

            result = pulumi.Output(lift(set()), lift(None), lift(False), lift(False))

        # If the value is a ResourceReference, we need to return a DependencyResource
        elif isinstance(value, ResourceReference):
            result = DependencyResource(value.urn)

        # For list and map values we want to return normal lists and dicts, not the internal tuple and proxy
        # types, we also need to recursively deserialize the values
        elif isinstance(value, Sequence) and not isinstance(value, str):
            result = []
            for item in value:
                result.append(item.deserialize())
        elif isinstance(value, Mapping):
            result = {}
            for key, item in value.items():
                result[key] = item.deserialize()

        # All other values can stay as they are, plain strings, assets, etc

        if value.is_secret:
            result = pulumi.Output.secret(result)

        if value.dependencies:
            deps = set(
                cast(Resource, DependencyResource(urn)) for urn in value.dependencies
            )
            result = pulumi.Output.from_input(result).with_dependencies(deps)

        return result

    def deserialize_map(
        self,
        value: Mapping[str, "PropertyValue"],
    ) -> Dict[str, pulumi.Input[Any]]:
        """
        Deserializes a dictionary of PropertyValues into a dictionary of Python values.

        :param value: The dictionary of PropertyValues to deserialize.
        :return: A dictionary of Python values.
        """
        result: Dict[str, Any] = {}
        for key, item in value.items():
            result[key] = self.deserialize(item)
        return result

    def serialize(self, value: Any) -> "PropertyValue":
        """
        Serializes a Python value into a PropertyValue.

        :param value: The Python value to serialize.
        :return: A PropertyValue representation of the value.
        """
        if isinstance(value, PropertyValue):
            return value
        elif isinstance(value, bool):
            return PropertyValue(value)
        elif isinstance(value, float):
            return PropertyValue(value)
        elif isinstance(value, str):
            return PropertyValue(value)
        elif isinstance(value, pulumi.Asset):
            return PropertyValue(value)
        elif isinstance(value, pulumi.Archive):
            return PropertyValue(value)
        elif isinstance(value, Sequence):
            # If the value is a sequence, we need to serialize each item in the sequence.
            return PropertyValue([self.serialize(item) for item in value])
        elif isinstance(value, Mapping):
            # If the value is a mapping, we need to serialize each item in the mapping.
            return PropertyValue(self.serialize_map(value))

        raise ValueError(
            f"Unsupported value type: {type(value)}. Supported types are: bool, float, str, Asset, Archive, Sequence, Mapping."
        )

    def serialize_map(
        self,
        value: pulumi.Inputs,
    ) -> Dict[str, "PropertyValue"]:
        """
        Serializes a dictionary of Python values into dictionary of PropertyValues.

        :param value: The dictionary of python values to serialize.
        :return: A dictionary of PropertyValues values.
        """
        result: Dict[str, Any] = {}
        for key, item in value.items():
            result[key] = self.serialize(item)
        return result
