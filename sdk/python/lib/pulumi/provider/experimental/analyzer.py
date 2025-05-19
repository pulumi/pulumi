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

import ast
import builtins
import collections
import importlib.resources
import inspect
import json
import sys
import types
import typing
from collections import abc
from collections.abc import Awaitable
from dataclasses import dataclass
from enum import Enum
from pathlib import Path
from types import GenericAlias
from typing import (  # type: ignore
    Any,
    ForwardRef,
    Literal,
    Optional,
    TypedDict,
    Union,
    _GenericAlias,  # type: ignore
    _SpecialGenericAlias,  # type: ignore
    cast,
    get_args,
    get_origin,
)

from ...asset import Archive, Asset
from ...output import Output
from ...resource import ComponentResource, Resource
from .util import camel_case

_NoneType = type(None)  # Available as typing.NoneType in >= 3.10

# Types for parameterized generics got a clean public API in 3.9 with
# https://peps.python.org/pep-0585/.
# The modern types like `dict[str, int]` use `GenericAlias`, but some
# decprecated types in the `typing` modoule use `typing._GenericAlias` or
# `typing._SpecialGenericAlias`.
_GenericAliasT = (  # type: ignore
    _GenericAlias,
    _SpecialGenericAlias,
    GenericAlias,
)


class PropertyType(Enum):
    STRING = "string"
    INTEGER = "integer"
    NUMBER = "number"
    BOOLEAN = "boolean"
    OBJECT = "object"
    ARRAY = "array"


@dataclass
class EnumValueDefinition:
    name: str
    value: Union[str, float, int, bool]
    description: Optional[str] = None


@dataclass
class PropertyDefinition:
    optional: bool = False
    type: Optional[PropertyType] = None
    ref: Optional[str] = None
    description: Optional[str] = None
    items: Optional["PropertyDefinition"] = None
    additional_properties: Optional["PropertyDefinition"] = None
    plain: Optional[Literal[True]] = None


@dataclass
class TypeDefinition:
    name: str
    type: PropertyType
    properties: dict[str, PropertyDefinition]
    properties_mapping: dict[str, str]
    """Mapping from the schema name to the Python name."""
    module: str
    """The Python module where this type is defined."""
    python_type: builtins.type
    """The Python type from which we derived this type definition."""
    description: Optional[str] = None
    enum: Optional[list[EnumValueDefinition]] = None


@dataclass
class ComponentDefinition:
    name: str
    inputs: dict[str, PropertyDefinition]
    outputs: dict[str, PropertyDefinition]
    inputs_mapping: dict[str, str]
    """Mapping from the schema name to the Python name."""
    outputs_mapping: dict[str, str]
    """Mapping from the schema name to the Python name."""
    module: Optional[str]
    """The Python module where this component is defined."""
    description: Optional[str] = None


@dataclass(frozen=True)
class Parameterization:
    name: str
    version: str
    # The value is a represented as base64 encoded string in JSON. Since all we
    # do is return it again in the schema, we don't decode it to bytes, and keep
    # it in base64.
    value: str


@dataclass(frozen=True)
class Dependency:
    name: str
    version: Optional[str] = None
    downloadURL: Optional[str] = None
    parameterization: Optional[Parameterization] = None


class TypeNotFoundError(Exception):
    def __init__(self, name: str):
        self.name = name
        super().__init__(
            f"Could not find the type '{name}'. "
            + "Ensure it is defined in your source code or is imported."
        )


class DuplicateTypeError(Exception):
    def __init__(
        self, new_module: str, existing: Union[TypeDefinition, ComponentDefinition]
    ):
        self.new_module = new_module
        self.existing = existing
        super().__init__(
            f"Duplicate type '{existing.name}': orginally defined in '{existing.module}', but also found in '{new_module}'"
        )


class InvalidMapKeyError(Exception):
    def __init__(self, key_type: type, typ: type, property_name: str):
        self.key = key_type
        self.property = property_name
        self.typ = typ
        super().__init__(
            f"map keys must be strings, got '{key_type.__name__}' for '{typ.__name__}.{property_name}'"
        )


class InvalidMapTypeError(Exception):
    def __init__(self, arg: type, typ: type, property_name: str):
        self.property = property_name
        self.typ = typ
        super().__init__(
            f"map types must specify two type arguments, got '{arg.__name__}' for '{typ.__name__}.{property_name}'"
        )


class InvalidListTypeError(Exception):
    def __init__(self, arg: type, typ: type, property_name: str):
        self.property = property_name
        self.typ = typ
        super().__init__(
            f"list types must specify a type argument, got '{arg.__name__}' for '{typ.__name__}.{property_name}'"
        )


class DependencyError(Exception): ...


class AnalyzeResult(TypedDict):
    component_definitions: dict[str, ComponentDefinition]
    """The components defined by the package."""
    type_definitions: dict[str, TypeDefinition]
    """The types defined in the package, these are complex types or enums."""
    external_enum_types: dict[str, type[Enum]]
    """
    A map of references to Python enum types. These are used to deserialize
    raw values back into Python types.
    """
    dependencies: set[Dependency]
    """The packages this package depends on."""


class Analyzer:
    """
    Analyzer infers component and type definitions for a set of
    ComponentResources types using type annotations.

    The entrypoint for this is `Analyzer.analyze`, which returns a dictionary of
    `ComponentDefinition`, which represent components, and a dictionary of
    `TypeDefinition`, which represent complex types, aka user defined types,
    used in the components inputs and/or outputs. This relies on a couple of
    assumptions:

    * Components are defined at the top level of the Python modules. Classes
      defined in a nested scope, such as a function, are not discovered.
      Essentially the analyser iterates over each element in `dir(module)` and
      looks for the subclasses at that level.
    * The underlying types in `ForwardRef`s are imported in one of the modules
      that are being analyzed.
    * The `__init__` method for each component has a typed argument named `args`
      which represent the inputs the component takes.
    * The types are put in a single Pulumi module, `index`. That is, all the Pulumi
      types have the typestring `<provider>:index:<type>`. This means that it is
      possible to have duplicate types, which raises an error durign analysis.

    To infer the schema, the analyzer follows the graph of types rooted at each
    component. From the component, it follows the `args` argument, and then
    follows each property of the args type. To implement recursive complex
    types, you have to use `ForwardRef`s
    https://docs.python.org/3/library/typing.html#typing.ForwardRef. The type a
    ForwardRef references is a string, which prevents us from following the
    "type pointer" to analyze it. If at the end of the analysis of a component
    we have unresolved forward references, the analyser resolves these by
    iterating over the Python modules in the same manner as it does to find the
    components.

    The type and property descriptions are inferred from the docstrings of the
    Python classes and their attributes. For classes, the docstrings are
    available on type.__doc__. Unfortunately, the docstrings of the class
    attributes are not present at runtime. We use inspect.getsource to get the
    source code and parse that to retrieve the docstrings.
    """

    def __init__(self, name: str):
        self.name = name
        self.dependencies: set[Dependency] = set()
        self.type_definitions: dict[str, TypeDefinition] = {}
        # Keep track of external enum types we encountered. We use these to map a schema
        # reference to a type so that we can deserialize a raw value back into a Python
        # enum type.
        self.external_enum_types: dict[str, type[Enum]] = {}
        # For unresolved types, we need to keep track of whether we saw them in a
        # component output or an input.
        self.unresolved_forward_refs: dict[
            str,
            tuple[
                bool,  # is_component_output
                TypeDefinition,
            ],
        ] = {}
        self.docstrings: dict[str, dict[str, str]] = {}

    def analyze(
        self,
        components: list[type[ComponentResource]],
    ) -> AnalyzeResult:
        """
        Analyze builds a Pulumi schema for a provider that handles the passed
        list of components.
        """
        component_defs: dict[str, ComponentDefinition] = {}
        for component in components:
            if component.__name__ in component_defs:
                raise DuplicateTypeError(
                    component.__module__, component_defs[component.__name__]
                )
            c = self.analyze_component(component)
            component_defs[c.name] = c

        # Look for any forward references we could not resolve in the first
        # pass. This happens for types that are only ever referenced in
        # ForwardRefs.
        # We expect the types to be imported in one of the modules of the
        # component we are analyzing.
        # With https://peps.python.org/pep-0649/ we might be able to let
        # Python handle this for us.
        checked_modules: set[str] = set()
        for name, (is_component_output, type_def) in [
            *self.unresolved_forward_refs.items()
        ]:
            for component in components:
                if component.__module__ in checked_modules:
                    continue
                checked_modules.add(component.__module__)
                module_name = component.__module__
                module = sys.modules.get(module_name)
                if module:
                    typ = getattr(module, type_def.name, None)
                    if typ:
                        (properties, properties_mapping) = self.analyze_type(
                            typ,
                            is_component_output=is_component_output,
                        )
                        type_def.properties = properties
                        type_def.properties_mapping = properties_mapping
                        type_def.python_type = typ
                        del self.unresolved_forward_refs[name]
                        break

        if len(self.unresolved_forward_refs) > 0:
            first_unresolved_ref = next(iter(self.unresolved_forward_refs))
            raise TypeNotFoundError(first_unresolved_ref)

        if len(components) == 0:
            raise Exception("No components found")

        return {
            "component_definitions": component_defs,
            "type_definitions": self.type_definitions,
            "dependencies": self.dependencies,
            "external_enum_types": self.external_enum_types,
        }

    def get_annotations(self, o: Any) -> dict[str, Any]:
        """
        Get the type annotations for `o` in a backwards compatible way.
        """
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
        """
        Analyze a single component, building up a `ComponentDefinition` that
        holds all the information necessary to create a resource in a Pulumi
        provider schema.
        """
        ann = self.get_annotations(component.__init__)
        args = ann.get("args", None)
        if not args:
            raise Exception(
                f"ComponentResource '{component.__name__}' requires an argument named 'args' with a type annotation in its __init__ method"
            )

        (inputs, inputs_mapping) = self.analyze_type(args, is_component_output=False)
        (outputs, outputs_mapping) = self.analyze_type(
            component, is_component_output=True
        )
        return ComponentDefinition(
            name=component.__name__,
            description=component.__doc__.strip() if component.__doc__ else None,
            inputs=inputs,
            inputs_mapping=inputs_mapping,
            outputs=outputs,
            outputs_mapping=outputs_mapping,
            module=component.__module__,
        )

    def analyze_type(
        self, typ: type, *, is_component_output: bool
    ) -> tuple[dict[str, PropertyDefinition], dict[str, str]]:
        """
        analyze_type returns a dictionary of the properties of a type based on
        its annotations, as well as a mapping from the schema property name
        (camel cased) to the Python property name.

        :param typ: the type to analyze
        :param is_component_output: whether the type is a used as a component
        output. Types used in component outputs can never have the plain
        property, including nested complex types or in list/dictionaries.

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
            camel_case(k): self.analyze_property(
                v,
                typ,
                k,
                can_be_plain=not is_component_output,
                is_component_output=is_component_output,
            )
            for k, v in ann.items()
        }, mapping

    def analyze_property(
        self,
        arg: type,
        typ: type,
        name: str,
        *,
        can_be_plain: bool,
        is_component_output: bool,
        optional: Optional[bool] = None,
    ) -> PropertyDefinition:
        """
        analyze_property analyzes a single annotation and turns it into a SchemaProperty.

        :param arg: the type of the property we are analyzing
        :param typ: the type this property belongs to
        :param name: the name of the property
        :param can_be_plain: whether the property can be plain
        :param is_component_output: whether the property is in a component output
        :param optional: whether the property is optional or not

        For properties of a type that's used as a component output, can_be_plain
        is always false. For properties used in an input, can_be_plain is true,
        indicating that they can potentially be plain.
        """
        optional = optional if optional is not None else is_optional(arg)
        if is_simple(arg):
            return PropertyDefinition(
                type=py_type_to_property_type(arg),
                optional=optional,
                # We are currently looking at a plain type, but it might be
                # wrapped in a pulumi.Input or pulumi.Output, in which case this
                # isn't plain.
                plain=true_or_none(can_be_plain),
                description=self.get_docstring(typ, name),
            )
        elif is_input(arg):
            return self.analyze_property(
                unwrap_input(arg),
                typ,
                name,
                can_be_plain=False,  # Property is inside an input -> it can't be plain
                is_component_output=is_component_output,
                optional=optional,
            )
        elif is_output(arg):
            return self.analyze_property(
                unwrap_output(arg),
                typ,
                name,
                can_be_plain=False,  # Property is inside an output -> it can't be plain
                is_component_output=is_component_output,
                optional=optional,
            )
        elif is_optional(arg):
            return self.analyze_property(
                unwrap_optional(arg),
                typ,
                name,
                can_be_plain=can_be_plain,
                is_component_output=is_component_output,
                optional=True,
            )
        elif is_any(arg):
            return PropertyDefinition(
                ref="pulumi.json#/Any",
                optional=optional,
                plain=true_or_none(can_be_plain),
                description=self.get_docstring(typ, name),
            )

        elif is_list(arg):
            args = get_args(arg)
            if len(args) != 1:
                raise InvalidListTypeError(arg, typ, name)
            items = self.analyze_property(
                args[0],
                typ,
                name,
                # The type of the list's items can potentially be plain, unless
                # we're in a type that's used as a component output.
                can_be_plain=not is_component_output,
                is_component_output=is_component_output,
            )
            return PropertyDefinition(
                type=PropertyType.ARRAY,
                optional=optional,
                plain=true_or_none(can_be_plain),
                items=items,
                description=self.get_docstring(typ, name),
            )
        elif is_dict(arg):
            args = get_args(arg)
            if len(args) != 2:
                raise InvalidMapTypeError(arg, typ, name)
            if args[0] is not str:
                raise InvalidMapKeyError(args[0], typ, name)
            return PropertyDefinition(
                type=PropertyType.OBJECT,
                optional=optional,
                plain=true_or_none(can_be_plain),
                additional_properties=self.analyze_property(
                    args[1],
                    typ,
                    name,
                    # The type of the dictionary's values can potentially be
                    # plain, unless we're in a type that's used as a component
                    # output.
                    can_be_plain=not is_component_output,
                    is_component_output=is_component_output,
                ),
                description=self.get_docstring(typ, name),
            )
        elif is_forward_ref(arg):
            ref_name = cast(ForwardRef, arg).__forward_arg__
            type_def = self.type_definitions.get(ref_name)
            # Forward references are assumed to be in the type's module.
            module = typ.__module__
            if type_def:
                if type_def.module != module:
                    raise DuplicateTypeError(module, type_def)
                # Forward ref to a type we saw before, return a reference to it.
                ref = f"#/types/{self.name}:index:{ref_name}"
                return PropertyDefinition(
                    ref=ref,
                    optional=optional,
                    plain=true_or_none(can_be_plain),
                    description=self.get_docstring(typ, name),
                )
            else:
                # Forward ref to a type we haven't seen yet. We create an empty
                # TypeDefiniton for it, and a return a PropertyDefinition that
                # references it. We also add it to the list of unresolved
                # forward references, so that we can come back to it after the
                # analysis is done.
                type_def = TypeDefinition(
                    name=ref_name,
                    type=PropertyType.OBJECT,
                    properties={},
                    properties_mapping={},
                    module=module,
                    description=self.get_docstring(typ, name),
                    python_type=arg,
                )
                self.unresolved_forward_refs[ref_name] = (is_component_output, type_def)
                self.type_definitions[type_def.name] = type_def
                ref = f"#/types/{self.name}:index:{type_def.name}"
                return PropertyDefinition(
                    ref=ref,
                    optional=optional,
                    plain=true_or_none(can_be_plain),
                    description=self.get_docstring(typ, name),
                )
        elif is_asset(arg):
            return PropertyDefinition(
                ref="pulumi.json#/Asset",
                optional=optional,
                description=self.get_docstring(typ, name),
            )
        elif is_archive(arg):
            return PropertyDefinition(
                ref="pulumi.json#/Archive",
                optional=optional,
                description=self.get_docstring(typ, name),
            )
        elif is_resource(arg):
            resource_type_string, package_name = get_package_name(arg, typ, name)
            try:
                dep = get_dependency_for_type(arg)
                self.dependencies.add(dep)
                return PropertyDefinition(
                    ref=f"/{dep.name}/v{dep.version}/schema.json#/resources/{resource_type_string.replace('/', '%2F')}",
                    optional=optional,
                    description=self.get_docstring(typ, name),
                )
            except DependencyError as e:
                raise Exception(f"{package_name}: {str(e)}")
        elif is_union(arg):
            raise Exception(
                f"Union types are not supported: found type '{arg}' for '{typ.__name__}.{name}'"
            )
        elif is_enum(arg):
            enum_type_string = getattr(arg, "pulumi_type", None)
            if enum_type_string:  # This is an enum from an external package
                _, package_name = get_package_name(arg, typ, name)
                try:
                    dep = get_dependency_for_type(arg)
                    self.dependencies.add(dep)
                    ref = f"/{dep.name}/v{dep.version}/schema.json#/types/{enum_type_string.replace('/', '%2F')}"
                    self.external_enum_types[ref] = arg
                    return PropertyDefinition(
                        ref=ref,
                        optional=optional,
                        description=self.get_docstring(typ, name),
                    )
                except DependencyError as e:
                    raise Exception(f"{package_name}: {str(e)}")

            type_name = arg.__name__
            type_def = self.type_definitions.get(type_name)
            if not type_def:
                type_def = TypeDefinition(
                    name=type_name,
                    module=arg.__module__,
                    type=enum_value_type(arg),
                    properties={},
                    properties_mapping={},
                    description=arg.__doc__,
                    enum=enum_members(arg),
                    python_type=arg,
                )
                for member in type_def.enum or []:
                    member.description = self.get_docstring(arg, member.name)
                self.type_definitions[type_def.name] = type_def
            elif type_def.module and type_def.module != arg.__module__:
                raise DuplicateTypeError(arg.__module__, type_def)
            ref = f"#/types/{self.name}:index:{type_def.name}"
            return PropertyDefinition(
                ref=ref,
                optional=optional,
                plain=true_or_none(can_be_plain),
                description=self.get_docstring(typ, name),
            )
        elif not is_builtin(arg):
            # We have a custom type, analyze it recursively. Immediately add the
            # type definition to the list of type definitions, before calling
            # `analyze_type`, so we can resolve recursive forward references.
            type_name = arg.__name__
            type_def = self.type_definitions.get(type_name)
            if not type_def:
                type_def = TypeDefinition(
                    name=type_name,
                    type=PropertyType.OBJECT,
                    properties={},
                    properties_mapping={},
                    description=arg.__doc__,
                    module=arg.__module__,
                    python_type=arg,
                )
                self.type_definitions[type_def.name] = type_def
            else:
                if type_def.module and type_def.module != arg.__module__:
                    raise DuplicateTypeError(arg.__module__, type_def)
            (properties, properties_mapping) = self.analyze_type(
                arg, is_component_output=is_component_output
            )
            type_def.properties = properties
            type_def.properties_mapping = properties_mapping
            if type_def.name in self.unresolved_forward_refs:
                del self.unresolved_forward_refs[type_def.name]
            ref = f"#/types/{self.name}:index:{type_def.name}"
            return PropertyDefinition(
                ref=ref,
                optional=optional,
                plain=true_or_none(can_be_plain),
                description=self.get_docstring(typ, name),
            )
        else:
            raise ValueError(f"Unsupported type '{arg}' for '{typ.__name__}.{name}'")

    def get_docstring(self, typ: type, name: str) -> Optional[str]:
        """
        Returns the docstring for the property with the given name on the given type.

        :param typ: The type on which the property is defined.
        :param name: The name of the property.

        Property docstrings are not available at runtime. To find the docstring,
        we get the source code of the type and parse it using the ast module. We
        iterate over the statements in the class definition and look for
        `AnnAssign` nodes. These represent a property declaration with a type
        annotation. Any string literal following the property declaration is the
        docstring.
        """
        if typ.__name__ in self.docstrings:
            return self.docstrings.get(typ.__name__, {}).get(name, None)

        try:
            src = inspect.getsource(typ)
            mod = ast.parse(src)
            for stmt in mod.body:
                if isinstance(stmt, ast.ClassDef):
                    class_name = stmt.name
                    self.docstrings[class_name] = {}
                    it = iter(stmt.body)
                    while True:
                        try:
                            node = next(it)
                            # For argument types or complex types, we'll have
                            # assignments with type annotations (ast.AnnAssign):
                            #
                            #   class SomeArgs(TypedDict):
                            #     abc: str                  # <- ast.AnnAssign
                            #
                            # Here we have an `ast.AnnAssign` with the target `abc`.
                            # For Enums, we instead have assignments without annotations:
                            #
                            #   class SomeEnum(Enum):
                            #     abc = "abc"               # <- ast.Assign
                            #
                            # In this case we have an `ast.Assign`. Since Python supports
                            # multiple assignement, this node type has a list of targets.
                            # We are only interested in cases with exactly one.
                            if isinstance(node, ast.AnnAssign) or isinstance(
                                node, ast.Assign
                            ):
                                if isinstance(node, ast.AnnAssign):
                                    target_node: ast.expr = node.target
                                else:
                                    # We have an ast.Assign
                                    if len(node.targets) != 1:
                                        continue
                                    target_node = node.targets[0]
                                if isinstance(target_node, ast.Name):
                                    target = target_node.id
                                    # Look for a docstring right after the assignment
                                    node = next(it)
                                    if (
                                        isinstance(node, ast.Expr)
                                        and isinstance(node.value, ast.Constant)
                                        and isinstance(node.value.value, str)
                                    ):
                                        self.docstrings[class_name][target] = (
                                            node.value.value
                                        )
                                    else:
                                        # Push back the node if it's not a docstring
                                        it = iter([node] + list(it))
                        except StopIteration:
                            break
        except Exception:  # noqa
            pass

        return self.docstrings.get(typ.__name__, {}).get(name, None)


def get_package_name(arg: type, typ: type, name: str) -> tuple[str, str]:
    type_string = getattr(arg, "pulumi_type", None)
    if not type_string:
        mod = arg.__module__.split(".")[0]
        raise Exception(
            f"Can not determine resource reference for type '{arg.__name__}' used in '{typ.__name__}.{name}': "
            + f"'{arg.__name__}.pulumi_type' is not defined. This may be due to an outdated version of '{mod}'."
        )
    parts = type_string.split(":")
    if len(parts) != 3:
        raise Exception(
            f"invalid type string '{type_string}' for type '{arg}' used in '{typ.__name__}.{name}'"
        )
    return type_string, parts[0]


def get_dependency_for_type(arg: type) -> Dependency:
    try:
        root_mod = arg.__module__.split(".")[0]
        pluginJSON = (
            importlib.resources.files(root_mod)
            .joinpath("pulumi-plugin.json")
            .open("r")
            .read()
        )
        plugin = json.loads(pluginJSON)
        args = {"name": plugin["name"], "version": plugin["version"]}
        if "server" in plugin:
            args["downloadURL"] = plugin["server"]
        if "parameterization" in plugin:
            p = plugin["parameterization"]
            args["parameterization"] = Parameterization(
                p["name"], p["version"], p["value"]
            )
        return Dependency(**args)
    except FileNotFoundError as e:
        raise DependencyError("Could not load pulumi-plugin.json") from e
    except json.JSONDecodeError as e:
        raise DependencyError("Could not parse pulumi-plugin.json for package") from e


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


def is_union(typ: type):
    # Check for `Union[a, b]`
    if get_origin(typ) == Union:
        return True
    if sys.version_info >= (3, 10):
        # Check for `a | b`
        return get_origin(typ) == types.UnionType
    return False


def is_enum(typ: type):
    return issubclass(typ, Enum)


def is_simple(typ: type) -> bool:
    return typ in (str, int, float, bool)


def is_optional(typ: type) -> bool:
    """
    A type is optional if it is a union that includes NoneType.
    """
    if is_union(typ):
        return _NoneType in get_args(typ)
    return False


def is_any(typ: type) -> bool:
    return typ is Any


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
    if not is_union(typ):
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


def is_list(typ: type) -> bool:
    t: Optional[type] = typ
    if isinstance(typ, _GenericAliasT):
        t = get_origin(typ)
    return t in (
        list,
        abc.Sequence,
        abc.MutableSequence,
        collections.UserList,
        typing.List,
        typing.Sequence,
        typing.MutableSequence,
    )


def is_dict(typ: type) -> bool:
    t: Optional[type] = typ
    if isinstance(typ, _GenericAliasT):
        t = get_origin(typ)
    return t in (
        dict,
        abc.Mapping,
        abc.MutableMapping,
        collections.defaultdict,
        collections.OrderedDict,
        collections.UserDict,
        typing.Dict,
        typing.Mapping,
        typing.MutableMapping,
        typing.DefaultDict,
        typing.OrderedDict,
    )


def is_resource(typ: type) -> bool:
    try:
        return issubclass(typ, Resource)
    except TypeError:
        return False


def is_asset(typ: type) -> bool:
    try:
        return issubclass(typ, Asset)
    except TypeError:
        return False


def is_archive(typ: type) -> bool:
    try:
        return issubclass(typ, Archive)
    except TypeError:
        return False


def enum_value_type(enu: type) -> PropertyType:
    if not issubclass(enu, Enum):
        raise Exception(f"Invalid enum type {enu}.")
    member = next(iter(enu.__members__.values()))
    if isinstance(member.value, bool):
        return PropertyType.BOOLEAN
    elif isinstance(member.value, int):
        return PropertyType.INTEGER
    elif isinstance(member.value, float):
        return PropertyType.NUMBER
    elif isinstance(member.value, str):
        return PropertyType.STRING
    raise Exception(
        f"Invalid type for enum value '{enu.__name__}.{member.name}': '{type(member.value)}'. "
        + "Supported enum value types are bool, str, float and int."
    )


def enum_members(enu: type) -> list[EnumValueDefinition]:
    if not issubclass(enu, Enum):
        raise Exception(f"Invalid enum type {enu}.")
    return [
        EnumValueDefinition(name=name, value=enum_value.value)
        for (name, enum_value) in enu.__members__.items()
    ]


def true_or_none(plain: bool) -> Union[Literal[True], None]:
    """Helper to set PropertyDefinition.plain. We want to omit this property if plain is false."""
    return True if plain else None
