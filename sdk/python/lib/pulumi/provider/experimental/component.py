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
from .analyzer import Analyzer, ComponentDefinition, PropertyDefinition, TypeDefinition
from .schema import generate_schema


class ComponentProvider(Provider):
    """
    ComponentProvider is a Pulumi provider that finds components from Python
    source code and infers a schema.
    """

    path: Path
    """The path to the Python source code."""

    _type_defs: dict[str, TypeDefinition]
    _component_defs: dict[str, ComponentDefinition]
    _components: dict[str, type[ComponentResource]]
    _name: str

    def __init__(
        self,
        components: list[type[ComponentResource]],
        name: str,
        namespace: Optional[str] = None,
        version: str = "0.0.0",
    ) -> None:
        self._name = name
        self.analyzer = Analyzer(name)
        (components_defs, type_definitions) = self.analyzer.analyze(components)
        self._components = {component.__name__: component for component in components}
        self._component_defs = components_defs
        self._type_defs = type_definitions
        schema = generate_schema(
            name,
            version,
            namespace,
            self._component_defs,
            self._type_defs,
        )
        super().__init__(version, json.dumps(schema.to_json()))

    def construct(
        self,
        name: str,
        resource_type: str,
        inputs: Inputs,
        options: Optional[ResourceOptions] = None,
    ) -> ConstructResult:
        self.validate_resource_type(self._name, resource_type)
        component_name = resource_type.split(":")[-1]
        constructor = self._components[component_name]
        component_def = self._component_defs[component_name]
        mapped_args = self.map_inputs(inputs, component_def)
        # Wrap the call to the component constuctor in a try except block to
        # catch any exceptions, so that we can re-raise a ComponentInitError.
        # This allows us to detect and report errors that occur within the user
        # code vs errors that occur in the SDK.
        try:
            # ComponentResource's init signature is different from the derived class signature.
            comp_instance = constructor(name, mapped_args, options)  # type: ignore
        except Exception as e:  # noqa
            raise ComponentInitError(e)
        state = self.get_state(comp_instance, component_def)
        return ConstructResult(comp_instance.urn, state)

    def get_type_definition(self, prop: PropertyDefinition) -> Optional[TypeDefinition]:
        """
        Gets the type definition for a property with a type reference.

        Returns None for built-in types like Asset and Archive.
        """
        if not prop.ref:
            raise ValueError(f"property {prop} is not a complex type")

        # Handle built-in types that don't have a colon in their reference
        # This includes types like Any, Asset, Archive, etc. (e.g. pulumi.json#/Asset)
        if ":" not in prop.ref:
            return None

        name = prop.ref.split(":")[-1]
        return self._type_defs[name]

    def map_inputs(self, inputs: Inputs, component_def: ComponentDefinition) -> Inputs:
        """
        Maps the input's names from the schema into Python names and
        validates that required inputs are present.
        """
        return self.map_input_properties(
            inputs,
            component_def.inputs,
            component_def.inputs_mapping,
            component_def.name,
            "",
        )

    def map_input_properties(
        self,
        inputs: Inputs,
        properties: dict[str, PropertyDefinition],
        mapping: dict[str, str],
        component_name: str,
        property_path: str,
    ) -> Inputs:
        """
        Generic helper to map property names from schema to Python names
        and validate required properties are present.

        This handles both top-level inputs and nested complex types.

        :param inputs: The inputs to map.
        :param properties: The property definitions that define the shape of the inputs.
        :param mapping: The mapping from schema property names to Python property names.
        :param component_name: The name of the component these inputs belong to.
        :param property_path: The path to the current property being mapped, used for error messages.
        """
        mapped_value: dict[str, Input[Any]] = {}
        for schema_name, prop in properties.items():
            input_val = (
                inputs.get(schema_name, None) if isinstance(inputs, dict) else None
            )
            if input_val is None:
                if not prop.optional:
                    full_path = (
                        schema_name
                        if not property_path
                        else f"{property_path}.{schema_name}"
                    )
                    raise InputPropertyError(
                        full_path,
                        f"Missing required input '{full_path}' on '{component_name}'",
                    )
                continue

            py_name = mapping[schema_name]

            # Handle complex types (named types)
            if prop.ref:
                # Get the type definition for the complex type
                type_def = self.get_type_definition(prop)
                if type_def is None:
                    mapped_value[py_name] = input_val
                    continue

                if type_def.enum:
                    try:
                        mapped_value[py_name] = type_def.python_type(input_val)
                    except ValueError as e:
                        full_path = (
                            schema_name
                            if not property_path
                            else f"{property_path}.{schema_name}"
                        )
                        raise InputPropertyError(
                            full_path,
                            f"Invalid value {input_val} of type {type(input_val)} for enum '{type_def.name}'",
                        ) from e
                    continue

                # Recursively map the complex type
                next_path = (
                    f"{property_path}.{schema_name}" if property_path else schema_name
                )
                mapped_value[py_name] = self.map_input_properties(
                    input_val if isinstance(input_val, dict) else {},
                    type_def.properties,
                    type_def.properties_mapping,
                    component_name,
                    next_path,
                )
                continue

            # Special handling for arrays of complex types
            if isinstance(input_val, list) and prop.items and prop.items.ref:
                type_def = self.get_type_definition(prop.items)
                if type_def is None:
                    mapped_value[py_name] = input_val
                    continue

                mapped_list = []
                for i, item in enumerate(input_val):
                    item_path = (
                        f"{property_path}.{schema_name}[{i}]"
                        if property_path
                        else f"{schema_name}[{i}]"
                    )
                    mapped_item = self.map_input_properties(
                        item,
                        type_def.properties,
                        type_def.properties_mapping,
                        component_name,
                        item_path,
                    )
                    mapped_list.append(mapped_item)

                mapped_value[py_name] = mapped_list
                continue

            # Handle dictionary of complex types
            if (
                isinstance(input_val, dict)
                and prop.additional_properties
                and prop.additional_properties.ref
            ):
                type_def = self.get_type_definition(prop.additional_properties)
                if type_def is None:
                    mapped_value[py_name] = input_val
                    continue

                mapped_dict = {}
                for key, value in input_val.items():
                    item_path = (
                        f"{property_path}.{schema_name}.{key}"
                        if property_path
                        else f"{schema_name}.{key}"
                    )
                    mapped_dict[key] = self.map_input_properties(
                        value,
                        type_def.properties,
                        type_def.properties_mapping,
                        component_name,
                        item_path,
                    )
                mapped_value[py_name] = mapped_dict
                continue

            # Simple type, just map the name
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
                # Get the type definition for the complex type
                type_def = self.get_type_definition(prop)
                if type_def is None:
                    state[k] = instance_val
                    continue
                if type_def.enum:
                    state[k] = instance_val.value
                    continue

                # It's a complex type, map it
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
                if nested_type_def is None:
                    r[schema_name] = val
                    continue
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
