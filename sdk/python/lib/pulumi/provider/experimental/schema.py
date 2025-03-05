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

from dataclasses import dataclass
from typing import Any, Optional, Union

from .component import (
    ComponentDefinition,
    PropertyDefinition,
    PropertyType,
    TypeDefinition,
)
from .metadata import Metadata


@dataclass
class ObjectType:
    """https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#objecttype"""

    type: PropertyType
    properties: dict[str, "Property"]
    required: list[str]
    # This has an optional description. However dataclasses dont't let us have
    # optional fields before non-optional fields, and since we inherit from this
    # class, we can't have this field here. Instead, the subclasses manually add
    # the description themselves.


@dataclass
class Property:
    """https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#property"""

    type: Optional[PropertyType]
    will_replace_on_changes: Optional[bool]
    items: Optional["Property"]
    additional_properties: Optional["Property"]
    ref: Optional[str]
    plain: Optional[bool]
    description: Optional[str] = None

    def to_json(self) -> dict[str, Any]:
        return {
            "description": self.description,
            "type": self.type.value if self.type else None,
            "willReplaceOnChanges": self.will_replace_on_changes
            if self.will_replace_on_changes
            else None,
            "items": self.items.to_json() if self.items else None,
            "additionalProperties": self.additional_properties.to_json()
            if self.additional_properties
            else None,
            "plain": self.plain if self.plain else None,
            "$ref": self.ref,
        }

    @staticmethod
    def from_definition(property: PropertyDefinition) -> "Property":
        return Property(
            description=property.description,
            type=property.type,
            will_replace_on_changes=False,
            items=Property.from_definition(property.items) if property.items else None,
            additional_properties=Property.from_definition(
                property.additional_properties
            )
            if property.additional_properties
            else None,
            ref=property.ref,
            plain=property.plain,
        )


@dataclass
class ComplexType(ObjectType):
    """https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#complextype"""

    description: Optional[str] = None
    enum: Optional[list[Any]] = None

    def to_json(self) -> dict[str, Any]:
        return {
            "type": self.type.value,
            "properties": {k: v.to_json() for k, v in self.properties.items()},
            "required": self.required,
            "description": self.description,
            "enum": self.enum,
        }

    @staticmethod
    def from_definition(
        type_def: TypeDefinition,
    ) -> "ComplexType":
        return ComplexType(
            type=PropertyType.OBJECT,
            properties={
                k: Property.from_definition(v) for k, v in type_def.properties.items()
            },
            required=sorted(
                [k for k, prop in type_def.properties.items() if not prop.optional]
            ),
            description=type_def.description,
        )


@dataclass
class Resource(ObjectType):
    """https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#resource"""

    is_component: bool
    input_properties: dict[str, Property]
    required_inputs: list[str]
    description: Optional[str] = None

    def to_json(self) -> dict[str, Any]:
        return {
            "isComponent": self.is_component,
            "description": self.description,
            "type": self.type.value,
            "inputProperties": {
                k: v.to_json() for k, v in self.input_properties.items()
            },
            "requiredInputs": self.required_inputs,
            "properties": {k: v.to_json() for k, v in self.properties.items()},
            "required": self.required,
        }

    @staticmethod
    def from_definition(component: ComponentDefinition) -> "Resource":
        return Resource(
            is_component=True,
            type=PropertyType.OBJECT,
            input_properties={
                k: Property.from_definition(property)
                for k, property in component.inputs.items()
            },
            required_inputs=sorted(
                [k for k, prop in component.inputs.items() if not prop.optional]
            ),
            properties={
                k: Property.from_definition(property)
                for k, property in component.outputs.items()
            },
            required=sorted(
                [k for k, prop in component.outputs.items() if not prop.optional]
            ),
        )


@dataclass
class PackageSpec:
    """https://www.pulumi.com/docs/iac/using-pulumi/pulumi-packages/schema/#package"""

    name: str
    displayName: str
    version: Optional[str]
    resources: dict[str, Resource]
    types: dict[str, ComplexType]
    language: dict[str, dict[str, Any]]

    def to_json(self) -> dict[str, Any]:
        return remove_none(
            {
                "name": self.name,
                "displayName": self.displayName,
                "version": self.version,
                "resources": {k: v.to_json() for k, v in self.resources.items()},
                "types": {k: v.to_json() for k, v in self.types.items()},
                "language": self.language,
            }
        )


def generate_schema(
    metadata: Metadata,
    components: dict[str, ComponentDefinition],
    type_definitions: dict[str, TypeDefinition],
) -> PackageSpec:
    pkg = PackageSpec(
        name=metadata.name,
        version=metadata.version,
        displayName=metadata.display_name or metadata.name,
        resources={},
        types={},
        language={
            "nodejs": {
                "respectSchemaVersion": True,
            },
            "python": {
                "respectSchemaVersion": True,
            },
            "csharp": {
                "respectSchemaVersion": True,
            },
            "java": {
                "respectSchemaVersion": True,
            },
            "go": {
                "respectSchemaVersion": True,
            },
        },
    )
    for component_name, component in components.items():
        name = f"{metadata.name}:index:{component_name}"
        pkg.resources[name] = Resource.from_definition(component)

    for type_name, type in type_definitions.items():
        pkg.types[f"{metadata.name}:index:{type_name}"] = ComplexType.from_definition(
            type
        )

    return pkg


def remove_none(d: Union[dict[str, Any], Any]) -> dict[str, Any]:
    if not isinstance(d, dict):
        return d
    return dict((k, remove_none(v)) for k, v in d.items() if v is not None)  # type: ignore
