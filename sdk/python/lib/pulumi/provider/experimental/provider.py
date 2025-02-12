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

import json
from pathlib import Path
from typing import Any, Optional, Union

from pulumi.provider.server import ComponentInitError

from ...errors import InputPropertyError
from ...output import Input, Inputs, Output
from ...resource import ComponentResource, ResourceOptions
from ..provider import ConstructResult, Provider
from .analyzer import Analyzer
from .component import ComponentDefinition, PropertyDefinition, TypeDefinition
from .metadata import Metadata
from .schema import generate_schema


class ComponentProvider(Provider):
    """
    ComponentProvider is a Pulumi provider that finds components from Python
    source code and infers a schema.
    """

    path: Path
    """The path to the Python source code."""
    metadata: Metadata
    """The metadata for the provider, such as the name and version."""

    _type_defs: dict[str, TypeDefinition]
    _component_defs: dict[str, ComponentDefinition]

    def __init__(self, metadata: Metadata, path: Path) -> None:
        self.path = path
        self.metadata = metadata
        self.analyzer = Analyzer(self.metadata)
        (components, type_definitions) = self.analyzer.analyze(self.path)
        self._component_defs = components
        self._type_defs = type_definitions
        schema = generate_schema(
            metadata,
            self._component_defs,
            self._type_defs,
        )
        super().__init__(metadata.version, json.dumps(schema.to_json()))

    def construct(
        self,
        name: str,
        resource_type: str,
        inputs: Inputs,
        options: Optional[ResourceOptions] = None,
    ) -> ConstructResult:
        self.validate_resource_type(self.metadata.name, resource_type)
        component_name = resource_type.split(":")[-1]
        comp = self.analyzer.find_type(self.path, component_name)
        component_def = self._component_defs[component_name]
        mapped_args = self.map_inputs(inputs, component_def)
        # Wrap the call to the component constuctor in a try except block to
        # catch any exceptions, so that we can re-raise a ComponentInitError.
        # This allows us to detect and report errors that occur within the user
        # code vs errors that occur in the SDK.
        try:
            # ComponentResource's init signature is different from the derived class signature.
            comp_instance = comp(name, mapped_args, options)  # type: ignore
        except Exception as e:  # noqa
            raise ComponentInitError(e)
        state = self.get_state(comp_instance, component_def)
        return ConstructResult(comp_instance.urn, state)

    def get_type_definition(self, prop: PropertyDefinition) -> TypeDefinition:
        """Gets the type definition for a property with a type reference."""
        if not prop.ref:
            raise ValueError(f"property {prop} is not a complex type")
        name = prop.ref.split(":")[-1]
        return self._type_defs[name]

    def map_inputs(self, inputs: Inputs, component_def: ComponentDefinition) -> Inputs:
        """
        Maps the input's names from the schema into Python names and
        validates that required inputs are present.
        """
        mapped_input: dict[str, Input[Any]] = {}
        for schema_name, prop in component_def.inputs.items():
            input_val = inputs.get(schema_name, None)
            if input_val is None:
                if not prop.optional:
                    raise InputPropertyError(
                        schema_name,
                        f"Missing required input '{schema_name}' on '{component_def.name}'",
                    )
                continue
            py_name = component_def.inputs_mapping[schema_name]
            if prop.ref:
                if prop.ref in ("pulumi.json#/Asset", "pulumi.json#/Archive"):
                    mapped_input[py_name] = input_val
                    continue
                type_def = self.get_type_definition(prop)
                mapped_input[py_name] = self.map_complex_input(
                    input_val,  # type: ignore
                    type_def,
                    component_def,
                    schema_name,
                )
            else:
                mapped_input[py_name] = input_val
        return mapped_input

    def map_complex_input(
        self,
        inputs: Inputs,
        type_def: TypeDefinition,
        component_def: ComponentDefinition,
        property_name: str,
    ) -> Inputs:
        """
        Recursively maps the names of a complex type from schema to Python names
        and validates that required inputs are present.

        :param inputs: The inputs for the complex type.
        :param type_def: The type definition for the complex type.
        :param component_def: The current component definition, which has a property of this complex type.
        :param property_name: The name of the property in the component definition that has this complex type.
        """
        mapped_value: dict[str, Input[Any]] = {}
        for schema_name, prop in type_def.properties.items():
            input_val = inputs.get(schema_name, None)
            if input_val is None:
                if not prop.optional:
                    property_path = f"{property_name}.{schema_name}"
                    raise InputPropertyError(
                        property_path,
                        f"Missing required input '{property_path}' on '{component_def.name}'",
                    )
                continue
            py_name = type_def.properties_mapping[schema_name]
            if prop.ref:
                # A nested complex type, get the type definition and recursively map it.
                nested_type_def = self.get_type_definition(prop)
                mapped_value[py_name] = self.map_complex_input(
                    input_val,  # type: ignore
                    nested_type_def,
                    component_def,
                    property_name + "." + schema_name,
                )
            else:
                mapped_value[py_name] = input_val
        return mapped_value

    def get_state(
        self, instance: ComponentResource, component_def: ComponentDefinition
    ) -> dict[str, Any]:
        state: dict[str, Any] = {}
        for k, prop in component_def.outputs.items():
            py_name = component_def.outputs_mapping[k]
            instance_val = getattr(instance, py_name, None)
            if instance_val is None:
                continue
            if prop.ref:
                if prop.ref in ("pulumi.json#/Asset", "pulumi.json#/Archive"):
                    state[k] = instance_val
                    continue
                # It's a complex type, get the type definition and map it
                type_def = self.get_type_definition(prop)
                state[k] = self.map_complex_output(instance_val, type_def)  # type: ignore
            else:
                state[k] = instance_val
        return state

    def map_complex_output(
        self,
        instance_val: Union[dict[str, Any], Output[dict[str, Any]]],
        type_def: TypeDefinition,
    ) -> Union[dict[str, Any], Output[dict[str, Any]]]:
        """Recursively maps the names of a complex type from Python to schema names."""
        # The complex type might be an Output. If so, we call the mapping
        # function in an apply.
        if isinstance(instance_val, Output):
            return instance_val.apply(lambda v: self.map_complex_output(v, type_def))

        r: dict[str, Any] = {}
        for schema_name, prop in type_def.properties.items():
            py_name = type_def.properties_mapping[schema_name]
            val = instance_val.get(py_name, None)
            if val is None:
                continue
            if prop.ref:
                nested_type_def = self.get_type_definition(prop)
                r[schema_name] = self.map_complex_output(val, nested_type_def)
            else:
                r[schema_name] = val
        return r

    @staticmethod
    def validate_resource_type(pkg_name: str, resource_type: str) -> None:
        parts = resource_type.split(":")
        if len(parts) != 3:
            raise ValueError(f"invalid resource type: {resource_type}")
        if parts[0] != pkg_name:
            raise ValueError(f"invalid provider: {parts[0]}, expected {pkg_name}")
        # We might want to relax this limitation, but for now we only support the "index" module.
        if parts[1] not in ["index", ""]:
            raise ValueError(
                f"invalid modle '{parts[1]}' in resource type: {resource_type}, expected index or empty string"
            )
        component_name = parts[2]
        if len(component_name) == 0:
            raise ValueError(f"empty component name in resource type: {resource_type}")
