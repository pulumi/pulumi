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
from typing import (
    Any,
    ForwardRef,
    Optional,
    Union,
    cast,
    get_args,
    get_origin,
)

from ... import log
from ...output import Output
from ...resource import ComponentResource
from .component import (
    ComponentDefinition,
    PropertyDefinition,
    PropertyType,
    TypeDefinition,
)
from .metadata import Metadata
from .util import camel_case

_NoneType = type(None)  # Available as typing.NoneType in >= 3.10


class TypeNotFoundError(Exception):
    def __init__(self, name: str):
        self.name = name
        super().__init__(f"Type '{name}' not found")


class Analyzer:
    def __init__(self, metadata: Metadata):
        self.metadata = metadata
        self.type_definitions: dict[str, TypeDefinition] = {}
        self.unresolved_forward_refs: dict[str, TypeDefinition] = {}

    def analyze(
        self, path: Path
    ) -> tuple[dict[str, ComponentDefinition], dict[str, TypeDefinition]]:
        """
        Analyze walks the directory at `path` and searches for
        ComponentResources in Python files.
        """
        components: dict[str, ComponentDefinition] = {}
        for file_path in self.iter(path):
            components.update(self.analyze_file(file_path))

        # Look for any forward references we could not resolve in the first
        # pass. This can happen when using mutually recursive types.
        # With https://peps.python.org/pep-0649/ we should be able to let
        # Python handle this for us.
        # This is a best effort attempt that handles common cases, but it is
        # possible to construct types we can't resolve.
        for name, type_def in [*self.unresolved_forward_refs.items()]:
            try:
                a = self.find_type(path, type_def.name)
                (properties, properties_mapping) = self.analyze_type(a)
                type_def.properties = properties
                type_def.properties_mapping = properties_mapping
                del self.unresolved_forward_refs[name]
            except TypeNotFoundError as e:
                log.warn(f"Could not resolve forward reference {name}: {e}")

        return (components, self.type_definitions)

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

    def find_type(self, path: Path, name: str) -> type:
        """
        Find a component by name in the directory at `self.path`.

        :param name: The name of the component to find.
        """
        for file_path in self.iter(path):
            mod = self.load_module(file_path)
            comp = getattr(mod, name, None)
            if comp:
                return comp
        raise TypeNotFoundError(name)

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

    def get_annotations(self, o: Any) -> dict[str, Any]:
        if sys.version_info >= (3, 10):
            # Only available in 3.10 and later
            return inspect.get_annotations(o)
        else:
            # On Python 3.9 and older, __annotations__ is not guaranteed to be
            # present. Additionally, if the class has no annotations, and it is
            # a subclass, it will return the annotations of the parent
            # https://docs.python.org/3/howto/annotations.html#accessing-the-annotations-dict-of-an-object-in-python-3-9-and-older
            if isinstance(o, type):
                return o.__dict__.get("__annotations__", {})
            else:
                return getattr(o, "__annotations__", {})

    def analyze_component(
        self, component: type[ComponentResource]
    ) -> ComponentDefinition:
        ann = self.get_annotations(component.__init__)
        args = ann.get("args", None)
        if not args:
            raise Exception(
                f"ComponentResource '{component.__name__}' requires an argument named 'args' with a type annotation in its __init__ method"
            )

        (inputs, inputs_mapping) = self.analyze_type(args)
        (outputs, outputs_mapping) = self.analyze_type(component)
        return ComponentDefinition(
            description=component.__doc__.strip() if component.__doc__ else None,
            inputs=inputs,
            inputs_mapping=inputs_mapping,
            outputs=outputs,
            outputs_mapping=outputs_mapping,
        )

    def analyze_type(
        self, typ: type
    ) -> tuple[dict[str, PropertyDefinition], dict[str, str]]:
        """
        analyze_type returns a dictionary of the properties of a type based on
        its annotations, as well as a mapping from the schema property name
        (camel cased) to the Python property name.

        For example for the class

            class SelfSignedCertificateArgs:
                algorithm: pulumi.Output[str]
                rsa_bits: Optional[pulumi.Output[int]]

        we get the following properties and mapping:

            (
                {
                    "algorithm": SchemaProperty(type=PropertyType.STRING, optional=False),
                    "rsaBits": SchemaProperty(type=PropertyType.INTEGER, optional=True)
                },
                {
                    "algorithm": "algorithm",
                    "rsaBits": "rsa_bits"
                }
            )
        """
        ann = self.get_annotations(typ)
        mapping: dict[str, str] = {camel_case(k): k for k in ann.keys()}
        return {
            camel_case(k): self.analyze_property(v) for k, v in ann.items()
        }, mapping

    def analyze_property(
        self, arg: type, optional: Optional[bool] = None
    ) -> PropertyDefinition:
        """
        analyze_property analyzes a single annotation and turns it into a SchemaProperty.
        """
        optional = optional if optional is not None else is_optional(arg)
        if is_plain(arg):
            # TODO: handle plain types
            return PropertyDefinition(
                type=py_type_to_property_type(arg),
                optional=optional,
            )
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
        elif is_forward_ref(arg):
            name = cast(ForwardRef, arg).__forward_arg__
            type_def = self.type_definitions.get(name)
            if type_def:
                # Forward ref to a type we saw before, return a reference to it.
                ref = f"#/types/{self.metadata.name}:index:{name}"
                return PropertyDefinition(
                    ref=ref,
                    optional=optional,
                )
            else:
                # Forward ref to a type we haven't seen yet. We create an empty
                # TypeDefiniton for it, and a return a PropertyDefinition that
                # references it. We also add it to the list of unresolved
                # forward references, so that we can come back to it after the
                # full analysis is done.
                type_def = TypeDefinition(
                    name=name,
                    type="object",
                    properties={},
                    properties_mapping={},
                )
                self.unresolved_forward_refs[name] = type_def
                self.type_definitions[type_def.name] = type_def
                ref = f"#/types/{self.metadata.name}:index:{type_def.name}"
                return PropertyDefinition(
                    ref=ref,
                    optional=optional,
                )
        elif not is_builtin(arg):
            # We have a custom type, analyze it recursively. Immediately add the
            # type definition to the list of type definitions, before calling
            # `analyze_type`, so we can resolve recursive forward references.
            name = arg.__name__
            type_def = self.type_definitions.get(name)
            if not type_def:
                type_def = TypeDefinition(
                    name=name,
                    type="object",
                    properties={},
                    properties_mapping={},
                    description=arg.__doc__,
                )
                self.type_definitions[type_def.name] = type_def
            (properties, properties_mapping) = self.analyze_type(arg)
            type_def.properties = properties
            type_def.properties_mapping = properties_mapping
            if type_def.name in self.unresolved_forward_refs:
                del self.unresolved_forward_refs[type_def.name]
            ref = f"#/types/{self.metadata.name}:index:{type_def.name}"
            return PropertyDefinition(
                ref=ref,
                optional=optional,
            )
        else:
            raise ValueError(f"unsupported type {arg}")


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
