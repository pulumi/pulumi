import ast
import importlib.util
import inspect
import sys
from collections.abc import Awaitable
from dataclasses import dataclass
from pathlib import Path
from types import ModuleType, NoneType
from typing import (
    ForwardRef,
    Optional,
    Union,
    get_args,
    get_origin,
)

import pulumi

from .metadata import Metadata
from .util import camel_case


@dataclass
class SchemaProperty:
    optional: bool = False
    type_: Optional[type] = None
    ref: Optional[str] = None
    description: Optional[str] = None


@dataclass
class TypeDefinition:
    name: str
    type: str
    properties: dict[str, SchemaProperty]
    description: Optional[str]


@dataclass
class ComponentSchema:
    inputs: dict[str, SchemaProperty]
    outputs: dict[str, SchemaProperty]
    description: Optional[str] = None


class Analyzer:
    def __init__(self, metadata: Metadata, path: Path):
        self.path = path
        self.metadata = metadata
        self.docstrings: dict[str, dict[str, str]] = {}
        self.type_definitions: dict[str, TypeDefinition] = {}

    def analyze(self) -> dict[str, ComponentSchema]:
        """
        analyze walks the directory at `self.path` and searches for
        ComponentResources in all the Python files.
        """
        self.docstrings = self.find_docstrings()
        return self.analyze_dir()

    def analyze_dir(self) -> dict[str, ComponentSchema]:
        components: dict[str, ComponentSchema] = {}
        for file_path in self.path.iterdir():
            if file_path.suffix != ".py":
                continue
            comps = self.analyze_file(file_path)
            components.update(comps)
        return components

    def analyze_file(self, file_path: Path) -> dict[str, ComponentSchema]:
        components: dict[str, ComponentSchema] = {}
        module_type = self.load_module(file_path)
        for name in dir(module_type):
            obj = getattr(module_type, name)
            if inspect.isclass(obj):
                if pulumi.ComponentResource in obj.__bases__:
                    component = self.analyze_component(obj)
                    components[self.arg_name(name)] = component
        return components

    def find_component(self, name: str) -> tuple[type[pulumi.ComponentResource], type]:
        """
        Find a component by name in the directory at `self.path` and return the
        ComponentResource class and its args class.
        """
        for file_path in self.path.iterdir():
            if file_path.suffix != ".py":
                continue
            mod = self.load_module(file_path)
            comp = getattr(mod, name, None)
            if not comp:
                continue
            # TODO: handle kwargs variant in addition to of args param? Args classes vs TypedDict?
            args = comp.__init__.__annotations__.get("args")
            return comp, args
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
        self,
        component: type[pulumi.ComponentResource],
    ) -> ComponentSchema:
        # TODO: handle kwargs variant in addition to of args param? Args classes vs TypedDict?
        args = component.__init__.__annotations__.get("args")
        if not args:
            raise Exception(f"Could not find in {component}'s __init__ method")
        return ComponentSchema(
            description=component.__doc__.strip() if component.__doc__ else None,
            inputs=self.analyze_types(args),
            outputs=self.analyze_types(component),
        )

    def analyze_component_outputs(
        self, component: type[pulumi.ComponentResource]
    ) -> list[str]:
        """Returns the names of the output properties of a component."""
        return list(self.analyze_types(component).keys())

    def analyze_types(self, typ: type) -> dict[str, SchemaProperty]:
        """
        analyze_types returns a dictionary of the properties of a type based on
        the its annotations.

        For example for the class

        class SelfSignedCertificateArgs:
            algorithm: pulumi.Output[str]
            ecdsa_curve: Optional[pulumi.Output[str]]

        we get

        {
            "algorithm": SchemaProperty(type=str, optional=False),
            "ecdsa_curve": SchemaProperty(type=str, optional=True)
        }
        """
        # TODO: should we use get_type_hints instead to resolve ForwardRefs?
        # What's the global context?
        # hints = get_type_hints(typ, globalns=globals())

        types = {}
        if not hasattr(typ, "__annotations__"):
            return types
        for k, v in typ.__annotations__.items():
            (schema_property, type_def) = self.analyze_arg(v)
            schema_property.description = self.docstrings.get(typ.__name__, {}).get(k)
            if type_def:
                self.type_definitions[type_def.name] = type_def
                ref = f"#/types/{self.metadata.name}:index:{type_def.name}"
                schema_property.ref = ref
            types[self.arg_name(k)] = schema_property
        return types

    def analyze_arg(
        self, arg: type, optional: Optional[bool] = None
    ) -> tuple[SchemaProperty, Optional[TypeDefinition]]:
        """
        analyze_arg analyzes a single annotation and turns it into a SchemaProperty.

        Any complex types are stored as TypeDefinitions in `self.type_definitions`.
        """
        optional = optional if optional is not None else is_optional(arg)
        unwrapped = None
        ref = None
        type_def = None
        if is_plain(arg):
            unwrapped = arg
        elif is_input(arg):
            return self.analyze_arg(unwrap_input(arg), optional=optional)
        elif is_output(arg):
            return self.analyze_arg(unwrap_output(arg), optional=optional)
        elif is_optional(arg):
            return self.analyze_arg(unwrap_optional(arg), optional=True)
        elif not is_builtin(arg):
            unwrapped = None
            type_def = TypeDefinition(
                name=self.arg_name(arg.__name__),
                type="object",
                properties=self.analyze_types(arg),
                description=arg.__doc__,
            )
        else:
            raise ValueError(f"Unsupported type {arg}")
        # TODO:
        #  * list property
        #  * dict/TypedDict

        return (
            SchemaProperty(
                type_=unwrapped,
                ref=ref,
                optional=optional,
            ),
            type_def,
        )

    def find_docstrings(self) -> dict[str, dict[str, str]]:
        """
        find_docstrings returns the docstrings for all the attributes of all
        the classes in `self.path.

        Unfortunately, only class docstrings are available at runtime, the
        docstrings of the attributes are not available. Instead of relying on
        runtime information we parse the source code to extract the docstrings.
        """
        docs = {}
        for file_path in self.path.iterdir():
            if file_path.suffix != ".py":
                continue
            with open(file_path) as f:
                src = f.read()
                t = ast.parse(src)
                docs.update(self.find_docstrings_in_module(t))
        return docs

    def find_docstrings_in_module(self, mod: ast.Module) -> dict[str, dict[str, str]]:
        docs: dict[str, dict[str, str]] = {}
        for stmt in mod.body:
            if isinstance(stmt, ast.ClassDef):
                class_name = stmt.name
                docs[class_name] = {}
                it = iter(stmt.body)
                while True:
                    try:
                        node = next(it)
                        # Look for an assignment with a type annotation
                        if isinstance(node, ast.AnnAssign):
                            if isinstance(node.target, ast.Name):
                                name = node.target.id
                                # Look for a docstring right after the assignment
                                node = next(it)
                                if isinstance(node, ast.Expr) and isinstance(
                                    node.value, ast.Str
                                ):
                                    docs[class_name][name] = node.value.value
                                else:
                                    # Push back the node if it's not a docstring
                                    it = iter([node] + list(it))
                    except StopIteration:
                        break

        return docs

    def arg_name(self, name: str) -> str:
        return camel_case(name)


def is_plain(typ: type) -> bool:
    return typ in (str, int, float, bool)


def is_optional(typ: type) -> bool:
    """
    A type is optional if it is a union that includes NoneType.
    """
    if get_origin(typ) == Union:
        return NoneType in get_args(typ)
    return False


def unwrap_optional(typ: type) -> type:
    """
    Returns the first type of the Union that is not NoneType.
    """
    if not is_optional(typ):
        raise ValueError("Not an optional type")
    elements = get_args(typ)
    for element in elements:
        if element is not NoneType:
            return element
    raise ValueError("Optional type with no non-None elements")


def is_output(typ: type):
    return get_origin(typ) == pulumi.Output


def unwrap_output(typ: type) -> type:
    if not is_output(typ):
        raise ValueError("Not an output type")
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
            # args = get_args(element)
            # base_type = args[0]
        elif is_output(element):
            has_output = True
        elif is_forward_ref(element):
            # In the core SDK, Input includes a forward reference to Output[T]
            if element.__forward_arg__ == "Output[T]":
                has_output = True
        else:
            has_plain = True

    # We could be stricter here and ensure that the base type used in Awaitable
    # and Output is the plain type.
    # For Output this is a little tricky, because it's a forward reference.
    # We can probably make that work using `get_type_hints`, but have to be careful
    # to set a global namespace that can resolve all the forward references,
    # including ones defined by the user.
    if has_awaitable and has_output and has_plain:
        return True

    return False


def unwrap_input(typ: type) -> type:
    if not is_input(typ):
        raise ValueError("Not an input type")
    # Look for the first Awaitable element and return its base type.
    for element in get_args(typ):
        if get_origin(element) is Awaitable:
            return get_args(element)[0]
    raise ValueError("Input type with no Awaitable elements")


def is_forward_ref(typ: type) -> bool:
    return isinstance(typ, ForwardRef)


def is_builtin(typ: type) -> bool:
    return typ.__module__ == "builtins"


def has_component(path: Path) -> bool:
    """
    has_component checks if the directory at `path` contains any ComponentResource.
    """
    analyzer = Analyzer(Metadata(path.absolute().name, "0.0.1"), path)
    return any(analyzer.analyze().values())
