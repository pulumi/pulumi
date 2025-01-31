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
from enum import Enum
from typing import Any, Optional

from .component import ComponentDefinition, PropertyDefinition
from .metadata import Metadata


class BuiltinType(Enum):
    STRING = "string"
    INTEGER = "integer"
    NUMBER = "number"
    BOOLEAN = "boolean"
    OBJECT = "object"


@dataclass
class ObjectType:
    type: BuiltinType
    properties: dict[str, "Property"]
    required: list[str]
    description: Optional[str] = None


@dataclass
class ItemType:
    type: BuiltinType

    def to_json(self) -> dict[str, str]:
        return {"type": str(self.type)}


@dataclass
class Property:
    description: Optional[str]
    type: Optional[BuiltinType]
    will_replace_on_changes: Optional[bool]
    items: Optional[ItemType]
    ref: Optional[str]

    def to_json(self) -> dict[str, Any]:
        return {
            "description": self.description,
            "type": self.type.value if self.type else None,
            "willReplaceOnChanges": self.will_replace_on_changes,
            "items": self.items,
            "$ref": self.ref,
        }

    @staticmethod
    def from_definition(property: PropertyDefinition) -> "Property":
        return Property(
            description=property.description,
            type=BuiltinType(property.type.value) if property.type else None,
            will_replace_on_changes=False,
            items=None,
            ref=property.ref,
        )


@dataclass
class Resource:
    is_component: bool
    input_properties: dict[str, Property]
    required_inputs: list[str]
    type_: BuiltinType
    properties: dict[str, Property]
    required: list[str]
    description: Optional[str] = None

    def to_json(self) -> dict[str, Any]:
        return {
            "isComponent": self.is_component,
            "description": self.description,
            "type": self.type_.value,
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
            type_=BuiltinType.OBJECT,
            input_properties={
                k: Property.from_definition(property)
                for k, property in component.inputs.items()
            },
            required_inputs=[
                k for k, prop in component.inputs.items() if not prop.optional
            ],
            properties={
                k: Property.from_definition(property)
                for k, property in component.outputs.items()
            },
            required=[k for k, prop in component.outputs.items() if not prop.optional],
        )


@dataclass
class PackageSpec:
    name: str
    displayName: str
    version: str
    resources: dict[str, Resource]
    language: dict[str, dict[str, Any]]

    def to_json(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "displayName": self.displayName,
            "version": self.version,
            "resources": {k: v.to_json() for k, v in self.resources.items()},
            "language": self.language,
        }


def generate_schema(
    metadata: Metadata, components: dict[str, ComponentDefinition]
) -> PackageSpec:
    pkg = PackageSpec(
        name=metadata.name,
        version=metadata.version,
        displayName=metadata.display_name or metadata.name,
        resources={},
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

    return pkg
