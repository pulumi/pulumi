from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from typing import Any, Optional

from .analyzer import Analyzer, SchemaProperty, TypeDefinition
from .metadata import Metadata


class BuiltinType(Enum):
    STRING = "string"
    INTEGER = "integer"
    NUMBER = "number"
    BOOLEAN = "boolean"
    OBJECT = "object"


@dataclass
class ObjectType:
    type: BuiltinType  # "object" or the underlying type of an enum
    properties: dict[str, "Property"]
    required: list[str]
    description: Optional[str] = None


@dataclass
class ComplexType(ObjectType):
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
    def from_analyzer(
        type_def: TypeDefinition,
    ) -> "ComplexType":
        type_def.properties
        return ComplexType(
            type=BuiltinType.OBJECT,
            properties={
                k: Property.from_analyzer(v) for k, v in type_def.properties.items()
            },
            required=[],
            description=type_def.description,
        )


@dataclass
class ItemType:
    type: str

    def to_json(self) -> dict[str, str]:
        return {"type": self.type}


@dataclass
class Property:
    description: Optional[str]
    type: Optional[str]
    will_replace_on_changes: Optional[bool]
    items: Optional[ItemType]
    ref: Optional[str]

    def to_json(self) -> dict[str, Any]:
        return {
            "description": self.description,
            "type": self.type,
            "willReplaceOnChanges": self.will_replace_on_changes,
            "items": self.items,
            "$ref": self.ref,
        }

    @staticmethod
    def from_analyzer(property: SchemaProperty) -> "Property":
        return Property(
            description=property.description,
            type=type_to_str(property.type_) if property.type_ else None,
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


@dataclass
class PackageSpec:
    name: str
    displayName: str
    version: str
    resources: dict[str, Resource]
    types: dict[str, ComplexType]
    language: dict[str, dict[str, Any]]

    def to_json(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "displayName": self.displayName,
            "version": self.version,
            "types": {k: v.to_json() for k, v in self.types.items()},
            "resources": {k: v.to_json() for k, v in self.resources.items()},
            "language": self.language,
        }


def type_to_str(typ: type) -> str:
    if typ is str:
        return "string"
    if typ is int:
        return "integer"
    if typ is float:
        return "number"
    if typ is bool:
        return "boolean"
    return "object"


def generate_schema(metadata: Metadata, path: Path) -> PackageSpec:
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
    a = Analyzer(metadata, path)
    components = a.analyze()
    for component_name, component in components.items():
        schema_name = f"{metadata.name}:index:{component_name}"
        pkg.resources[schema_name] = Resource(
            is_component=True,
            type_=BuiltinType.OBJECT,
            input_properties={
                k: Property.from_analyzer(property)
                for k, property in component.inputs.items()
            },
            required_inputs=[
                k for k, prop in component.inputs.items() if not prop.optional
            ],
            properties={
                k: Property.from_analyzer(property)
                for k, property in component.outputs.items()
            },
            required=[k for k, prop in component.outputs.items() if not prop.optional],
        )
    for type_name, type_ in a.type_definitions.items():
        pkg.types[f"{metadata.name}:index:{type_name}"] = ComplexType.from_analyzer(
            type_
        )

    return pkg
