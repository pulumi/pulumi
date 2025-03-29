import json
from pathlib import Path
from typing import Any, Optional, cast

import pulumi
from pulumi.provider import ConstructResult, Provider  # ParameterizeResult

from .analyzer import Analyzer
from .metadata import Metadata
from .schema import generate_schema
from .util import python_name


class ComponentProvider(Provider):
    path: Path

    def __init__(self, metadata: Metadata, path: Path) -> None:
        self.path = path
        self.metadata = metadata
        schema = generate_schema(metadata, path)
        super().__init__(metadata.version, json.dumps(schema.to_json()))

    # Needs implementation in the core SDK.
    # def parameterize_args(self, args: list[str]) -> ParameterizeResult:
    #     return ParameterizeResult(name=self.name, version=self.version)

    def construct(
        self,
        name: str,
        resource_type: str,
        inputs: pulumi.Inputs,
        options: Optional[pulumi.ResourceOptions] = None,
    ) -> ConstructResult:
        a = Analyzer(self.metadata, self.path)
        component_name = resource_type.split(":")[-1]
        comp, comp_args = a.find_component(component_name)
        args = comp_args(**{python_name(k): v for k, v in inputs.items()})
        comp_instance = cast(Any, comp)(name, args, options)
        keys = a.analyze_component_outputs(type(comp_instance))
        state = {
            a.arg_name(k): getattr(comp_instance, python_name(k), None) for k in keys
        }
        return ConstructResult(comp_instance.urn, state)
