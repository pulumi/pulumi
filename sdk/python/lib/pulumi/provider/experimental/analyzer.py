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

import importlib.util
import inspect
import sys
from collections.abc import Awaitable
from pathlib import Path
from types import ModuleType
from typing import Any, ForwardRef, Optional, Union, get_args, get_origin

from ...output import Output
from ...resource import ComponentResource
from .component import (
    ComponentDefinition,
    PropertyDefinition,
    PropertyType,
)
from .metadata import Metadata

_NoneType = type(None)  # Available as typing.NoneType in >= 3.10


class Analyzer:
    def __init__(self, metadata: Metadata):
        self.metadata = metadata

    def analyze(self, path: Path) -> dict[str, ComponentDefinition]:
        """
        Analyze walks the directory at `path` and searches for
        ComponentResources in Python files.
        """
        components: dict[str, ComponentDefinition] = {}
        for file_path in self.iter(path):
            components.update(self.analyze_file(file_path))
        return components

    def iter(self, path: Path):
        for file_path in path.glob("**/*.py"):
            if is_in_venv(file_path):
                continue
            yield file_path

    def analyze_file(self, file_path: Path) -> dict[str, ComponentDefinition]:
        components: dict[str, ComponentDefinition] = {}
        module_type = self.load_module(file_path)
        for name in dir(module_type):
            obj = getattr(module_type, name)
            if inspect.isclass(obj) and ComponentResource in obj.__bases__:
                components[name] = self.analyze_component(obj)
        return components

    def find_component(self, path: Path, name: str) -> type[ComponentResource]:
        """
        Find a component by name in the directory at `self.path`.

        :param name: The name of the component to find.
        """
        for file_path in self.iter(path):
            mod = self.load_module(file_path)
            comp = getattr(mod, name, None)
            if comp:
                return comp
        raise Exception(f"Could not find component {name}")

    def load_module(self, file_path: Path) -> ModuleType:
        name = file_path.name.replace(".py", "")
        spec = importlib.util.spec_from_file_location("component_file", file_path)
        if not spec:
            raise Exception(f"Could not load module spec at {file_path}")
        module_type = importlib.util.module_from_spec(spec)
        sys.modules[name] = module_type
        if not spec.loader:
            raise Exception(f"Could not load module at {file_path}")
        spec.loader.exec_module(module_type)
        return module_type

    def analyze_component(
        self, component: type[ComponentResource]
    ) -> ComponentDefinition:
        args = component.__init__.__annotations__.get("args", None)
        if not args:
            raise Exception(
                f"ComponentResource '{component.__name__}' requires an argument named 'args' with a type annotation in its __init__ method"
            )
        return ComponentDefinition(
            description=component.__doc__.strip() if component.__doc__ else None,
            inputs=self.analyze_type(args),
            outputs=self.analyze_type(component),
        )

    def analyze_type(self, typ: type) -> dict[str, PropertyDefinition]:
        """
        analyze_type returns a dictionary of the properties of a type based on
        its annotations.

        For example for the class

            class SelfSignedCertificateArgs:
                algorithm: pulumi.Output[str]
                bits: Optional[pulumi.Output[int]]

        we get the following properties:

            {
                "algorithm": SchemaProperty(type=PropertyType.STRING, optional=False),
                "bits": SchemaProperty(type=PropertyType.INTEGER, optional=True)
            }
        """
        if not hasattr(typ, "__annotations__"):
            return {}
        return {k: self.analyze_property(v) for k, v in typ.__annotations__.items()}

    def analyze_property(
        self, arg: type, optional: Optional[bool] = None
    ) -> PropertyDefinition:
        """
        analyze_property analyzes a single annotation and turns it into a SchemaProperty.
        """
        optional = optional if optional is not None else is_optional(arg)
        unwrapped = None
        ref = None
        if is_plain(arg):
            # TODO: handle plain types
            unwrapped = arg
        elif is_input(arg):
            return self.analyze_property(unwrap_input(arg), optional=optional)
        elif is_output(arg):
            return self.analyze_property(unwrap_output(arg), optional=optional)
        elif is_optional(arg):
            return self.analyze_property(unwrap_optional(arg), optional=True)
        elif isinstance(arg, list):
            raise ValueError("list types not yet implemented")
        elif isinstance(arg, dict):
            raise ValueError("dict types not yet implemented")
        elif not is_builtin(arg):
            raise ValueError("complex types not yet implemented")
        else:
            raise ValueError(f"unsupported type {arg}")

        return PropertyDefinition(
            type=py_type_to_property_type(unwrapped),
            ref=ref,
            optional=optional,
        )


def is_in_venv(path: Path):
    venv = Path(sys.prefix).resolve()
    path = path.resolve()
    return venv in path.parents


def py_type_to_property_type(typ: type) -> PropertyType:
    if typ is str:
        return PropertyType.STRING
    if typ is int:
        return PropertyType.INTEGER
    if typ is float:
        return PropertyType.NUMBER
    if typ is bool:
        return PropertyType.BOOLEAN
    return PropertyType.OBJECT


def is_plain(typ: type) -> bool:
    return typ in (str, int, float, bool)


def is_optional(typ: type) -> bool:
    """
    A type is optional if it is a union that includes NoneType.
    """
    if get_origin(typ) == Union:
        return _NoneType in get_args(typ)
    return False


def unwrap_optional(typ: type) -> type:
    """
    Returns the first type of the Union that is not NoneType.
    """
    if not is_optional(typ):
        raise ValueError("Not an optional type")
    elements = get_args(typ)
    for element in elements:
        if element is not _NoneType:
            return element
    raise ValueError("Optional type with no non-None elements")


def is_output(typ: type):
    return get_origin(typ) == Output


def unwrap_output(typ: type) -> type:
    """Get the base type of an Output[T]"""
    if not is_output(typ):
        raise ValueError(f"{typ} is not an output type")
    args = get_args(typ)
    return args[0]


def is_input(typ: type) -> bool:
    """
    An input type is a Union that includes Awaitable, Output and a plain type.
    """
    origin = get_origin(typ)
    if origin is not Union:
        return False

    has_awaitable = False
    has_output = False
    has_plain = False
    for element in get_args(typ):
        if get_origin(element) is Awaitable:
            has_awaitable = True
        elif is_output(element):
            has_output = True
        elif is_forward_ref(element) and element.__forward_arg__ == "Output[T]":
            # In the core SDK, Input includes a forward reference to Output[T]
            has_output = True
        else:
            has_plain = True

    # We could try to be stricter here and ensure that the base type used in
    # Awaitable and Output is the same as the plain type. However, since Output
    # is a ForwardRef it is tricky to determine its base type. We could
    # potentially attempt to load the types using get_type_hints into an
    # environment that allows resolving the ForwardRef.
    if has_awaitable and has_output and has_plain:
        return True

    return False


def unwrap_input(typ: type) -> type:
    """Get the base type of an Input[T]"""
    if not is_input(typ):
        raise ValueError(f"{typ} is not an input type")
    # Look for the first Awaitable element and return its base type.
    for element in get_args(typ):
        if get_origin(element) is Awaitable:
            return get_args(element)[0]
    # Not reachable, we checked above that it is an input, which requires an
    # `Awaitable` element.
    raise ValueError("Input type with no Awaitable elements")


def is_forward_ref(typ: Any) -> bool:
    return isinstance(typ, ForwardRef)


def is_builtin(typ: type) -> bool:
    return typ.__module__ == "builtins"
