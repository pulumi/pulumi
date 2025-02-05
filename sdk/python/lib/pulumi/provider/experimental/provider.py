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
from typing import Any, Optional

from ...output import Input, Inputs
from ...resource import ComponentResource, ResourceOptions
from ..provider import ConstructResult, Provider
from .analyzer import Analyzer
from .component import ComponentDefinition, TypeDefinition
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
        comp = self.analyzer.find_component(self.path, component_name)
        # Use the component definitions to map the schema names to the Python
        # names.
        # TODO: Handle complex types, including multiple levels of nesting.
        component_def = self._component_defs[component_name]
        mapped_args = self.map_input_names(inputs, component_def.inputs_mapping)
        # ComponentResource's init signature is different from the derived class signature.
        comp_instance = comp(name, mapped_args, options)  # type: ignore
        return ConstructResult(
            comp_instance.urn,
            self.get_state(comp_instance, component_def.outputs_mapping),
        )

    def map_input_names(self, inputs: Inputs, mapping: dict[str, str]) -> Inputs:
        r: dict[str, Input[Any]] = {}
        for k, v in inputs.items():
            r[mapping[k]] = v
        return r

    def get_state(
        self, instance: ComponentResource, mapping: dict[str, str]
    ) -> dict[str, Any]:
        state: dict[str, Any] = {}
        for k, v in mapping.items():
            state[k] = getattr(instance, v, None)
        return state

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
