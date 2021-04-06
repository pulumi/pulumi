# Copyright 2016-2020, Pulumi Corporation.
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

# This module exports decorators, functions, and other helpers for defining input/output types.
#
# A resource can be declared as:
#
#   class FooResource(pulumi.CustomResource):
#       nested_value: pulumi.Output[Nested] = pulumi.property("nestedValue")
#
#       def __init__(self, resource_name, nested_value: pulumi.InputType[NestedArgs]):
#           super().__init__("my:module:FooResource", resource_name, {"nestedValue": nested_value})
#
#
# The resource declares a single output `nested_value` of type `pulumi.Output[Nested]` and uses
# `pulumi.property()` to indicate the Pulumi property name.
#
# The resource's `__init__()` method accepts a `nested_value` argument typed as
# `pulumi.InputType[NestedArgs]`, which is an alias for accepting either an input type (in this
# case `NestedArgs`) or `Mapping[str, Any]`. Input types are converted to a `dict` during
# serialization.
#
# When the resource's outputs are resolved, the `Nested` class is instantiated.
#
# The resource could alternatively be declared using a Python property for nested_value rather than
# using a class annotation:
#
#   class FooResource(pulumi.CustomResource):
#       def __init__(self, resource_name, nested_value: pulumi.InputType[NestedArgs]):
#           super().__init__("my:module:FooResource", resource_name, {"nestedValue": nested_value})
#
#       @property
#       @pulumi.getter(name="nestedValue")
#       def nested_value(self) -> pulumi.Output[Nested]:
#           ...
#
#
# Note the `nested_value` property getter function is empty. The `@pulumi.getter` decorator replaces
# empty function bodies with an actual implementation. In this case, it replaces it with a body that
# looks like:
#
#       @property
#       @pulumi.getter(name="nestedValue")
#       def nested_value(self) -> pulumi.Output[Nested]:
#           pulumi.get(self, "nested_value")
#
#
# Here's how the `NestedArgs` input class can be declared:
#
#   @pulumi.input_type
#   class NestedArgs:
#       first_arg: pulumi.Input[str] = pulumi.property("firstArg")
#       second_arg: Optional[pulumi.Input[float]] = pulumi.property("secondArg", default=None)
#
#
# The class is decorated with the `@pulumi.input_type` decorator, which indicates the class is an
# input type and does some processing of the class (explained below). `NestedArgs` declares two
# inputs (`first_arg` and `second_arg`) and uses type annotations and `pulumi.property()` to
# specify the types and Pulumi input property names. An `__init__()` method is automatically added
# based on the annotations since one was not already present.
#
# A more verbose way to declare the same input type is as follows:
#
#   @pulumi.input_type
#   class NestedArgs:
#       def __init__(self, *, first_arg: pulumi.Input[str], second_arg: Optional[pulumi.Input[float]] = None):
#           pulumi.set(self, "first_arg", first_arg)
#           if second_arg is not None:
#               pulumi.set(self, "second_arg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> pulumi.Input[str]:
#           ...
#
#       @first_arg.setter
#       def first_arg(self, value: pulumi.Input[str]):
#           ...
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[pulumi.Input[float]]:
#           ...
#
#       @second_arg.setter
#       def second_arg(self, value: Optional[pulumi.Input[float]]):
#           ...
#
# This latter (more verbose) declaration is equivalent to the former (simpler) declaration;
# the `@pulumi.input_type` processes the class and transforms the former declaration into the
# latter declaration.
#
# The former (simpler) declaration is syntactic sugar to use when declaring these by hand,
# e.g. when writing a dynamic provider that has nested inputs/outputs. The latter declaration isn't
# as pleasant to write by hand and is closer to what we emit in our provider codegen. The benefit
# of the latter (more verbose) form is that it allows docstrings to be specified on the Python
# property getters, which will show up in IDE tooltips when hovering over the property.
#
# Note the property getter/setter functions are empty in the more verbose declaration.
# Empty getter functions are automatically replaced by the `@pulumi.getter` decorator with an
# actual implementation, and the `@pulumi.input_type` decorator will automatically replace any
# empty setter functions associated with a getter decorated with `@pulumi.getter` with an actual
# implementation. Thus, the above is equivalent to this even more verbose form:
#
#   @pulumi.input_type
#   class NestedArgs:
#       def __init__(self, *, first_arg: pulumi.Input[str], second_arg: Optional[pulumi.Input[float]] = None):
#           pulumi.set(self, "first_arg", first_arg)
#           if second_arg is not None:
#               pulumi.set(self, "second_arg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> pulumi.Input[str]:
#           return pulumi.get(self, "first_arg")
#
#       @first_arg.setter
#       def first_arg(self, value: pulumi.Input[str]):
#           pulumi.set(self, "first_arg", value)
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[pulumi.Input[float]]:
#           return pulumi.get(self, "second_arg")
#
#       @second_arg.setter
#       def second_arg(self, value: Optional[pulumi.Input[float]]):
#           pulumi.set(self, "second_arg", value)
#
#
# Here's how the `Nested` output class can be declared:
#
#   @pulumi.output_type
#   class Nested:
#       first_arg: str = pulumi.property("firstArg")
#       second_arg: Optional[float] = pulumi.property("secondArg")
#
#
# This is equivalent to the more verbose form:
#
#   @pulumi.output_type
#   class Nested:
#       def __init__(self, *, first_arg: str, second_arg: Optional[float]):
#           pulumi.set(self, "first_arg", first_arg)
#           pulumi.set(self, "second_arg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> str:
#           ...
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[float]:
#           ...
#
# An `__init__()` method is added to the class by the `@pulumi.output_type` decorator (if an
# `__init__()` method isn't already present on the class).
#
# Output types only have property getters and the bodies can be empty. Empty getter functions are
# replaced with implementations by the `@pulumi.getter` decorator.
#
# The above form is equivalent to:
#
#   @pulumi.output_type
#   class Nested:
#       def __init__(self, *, first_arg: str, second_arg: Optional[float]):
#           pulumi.set(self, "first_arg", first_arg)
#           pulumi.set(self, "second_arg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> str:
#           return pulumi.get(self, "first_arg")
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[float]:
#           return pulumi.get(self, "second_arg")
#
#
# Output classes can also be a subclass of `dict`. This is used in our provider codegen to maintain
# backwards compatibility, where previously these objects were instances of `dict`.
#
#   @pulumi.output_type
#   class Nested(dict):
#       first_arg: str = pulumi.property("firstArg")
#       second_arg: Optional[float] = pulumi.property("secondArg")
#
#
# The above output type, a subclass of `dict`, is equivalent to:
#
#   @pulumi.output_type
#   class Nested(dict):
#       def __init__(self, *, first_arg: str, second_arg: Optional[float]):
#           pulumi.set(self, "first_arg", first_arg)
#           pulumi.set(self, "second_arg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> str:
#           ...
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[float]:
#           ...
#
#
# Which is equivalent to:
#
#   @pulumi.output_type
#   class Nested(dict):
#       def __init__(self, *, first_arg: str, second_arg: Optional[float]):
#           pulumi.set(self, "first_arg", first_arg)
#           pulumi.set(self, "second_arg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> str:
#           return pulumi.get(self, "first_arg")
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[float]:
#           return pulumi.get(self, "second_arg")
#
#
# An output class can optionally include a `_translate_property(self, prop)` method, which
# `pulumi.get` and `pulumi.set` will call to translate the Pulumi property name to a translated
# key name before getting/setting the value in the dictionary. This is to provide backwards
# compatibility with our provider generated code, where mapping tables are used to translate dict
# keys before being returned to the program. This way, existing programs accessing the values as a
# dictionary will continue to see the same translated key names as before, but updated programs can
# now access the values using Python properties, which will always have thecorrect snake_case
# Python names.
#
#   @pulumi.output_type
#   class Nested(dict):
#       def __init__(self, *, first_arg: str, second_arg: Optional[float]):
#           pulumi.set(self, "first_arg", first_arg)
#           pulumi.set(self, "second_arg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> str:
#           ...
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[float]:
#           ...
#
#       def _translate_property(self, prop):
#           return _tables.CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop

import builtins
import collections.abc
import functools
import sys
import typing
from typing import Any, Callable, Dict, Iterator, Mapping, Optional, Tuple, Type, TypeVar, Union, cast, get_type_hints

from . import _utils

T = TypeVar('T')


_PULUMI_NAME = "_pulumi_name"
_PULUMI_INPUT_TYPE = "_pulumi_input_type"
_PULUMI_OUTPUT_TYPE = "_pulumi_output_type"
_PULUMI_PYTHON_TO_PULUMI_TABLE = "_pulumi_python_to_pulumi_table"
_TRANSLATE_PROPERTY = "_translate_property"


def is_input_type(cls: type) -> bool:
    return hasattr(cls, _PULUMI_INPUT_TYPE)

def is_output_type(cls: type) -> bool:
    return hasattr(cls, _PULUMI_OUTPUT_TYPE)


class _MISSING_TYPE:
    pass
MISSING = _MISSING_TYPE()
"""
MISSING is a singleton sentinel object to detect if a parameter is supplied or not.
"""

class _Property:
    """
    Represents a Pulumi property. It is not meant to be created outside this module,
    rather, the property() function should be used.
    """
    def __init__(self, name: str, default: Any = MISSING) -> None:
        if not name:
            raise TypeError("Missing name argument")
        if not isinstance(name, str):
            raise TypeError("Expected name to be a string")
        self.name = name
        self.default = default
        self.type: Any = None


# This function's return type is deliberately annotated as Any so that type checkers do not
# complain about assignments that we want to allow like `my_value: str = property("myValue")`.
# pylint: disable=redefined-builtin
def property(name: str, *, default: Any = MISSING) -> Any:
    """
    Return an object to identify Pulumi properties.

    name is the Pulumi property name.
    """
    return _Property(name, default)


def _properties_from_annotations(cls: type) -> Dict[str, _Property]:
    """
    Returns a dictionary of properties from annotations defined on the class.
    """

    # Get annotations that are defined on this class (not base classes).
    # These are returned in the order declared on Python 3.6+.
    cls_annotations = cls.__dict__.get('__annotations__', {})

    def get_property(cls: type, a_name: str, a_type: Any) -> _Property:
        default = getattr(cls, a_name, MISSING)
        p = default if isinstance(default, _Property) else _Property(name=a_name, default=default)
        p.type = a_type
        return p

    return {
        name: get_property(cls, name, type)
        for name, type in cls_annotations.items()
    }


def _process_class(cls: type, signifier_attr: str, is_input: bool = False, setter: bool = False):
    # Get properties.
    props = _properties_from_annotations(cls)

    # Clean-up class attributes.
    for name in props:
        # If the class attribute (which is the default value for this prop)
        # exists and is of type 'Property', delete the class attribute so
        # it is not set at all in the post-processed class.
        if isinstance(getattr(cls, name, None), _Property):
            delattr(cls, name)

    # Mark this class with the signifier and save the properties.
    setattr(cls, signifier_attr, True)

    # Create Python properties.
    for name, prop in props.items():
        setattr(cls, name, _create_py_property(name, prop.name, prop.type, setter))

    # Add an __init__() method if the class doesn't have one.
    if "__init__" not in cls.__dict__:
        if cls.__module__ in sys.modules:
            globals = sys.modules[cls.__module__].__dict__
        else:
            globals = {}
        init_fn = _init_fn(props, globals, issubclass(cls, dict), not is_input and hasattr(cls, _TRANSLATE_PROPERTY))
        setattr(cls, "__init__", init_fn)

    # Add an __eq__() method if the class doesn't have one.
    # There's no need for a __ne__ method, since Python will call __eq__ and negate it.
    if "__eq__" not in cls.__dict__:
        if issubclass(cls, dict):
            def eq_fn(self, other):
                return type(other) is type(self) and getattr(dict, "__eq__")(other, self)
        else:
            def eq_fn(self, other):
                return type(other) is type(self) and other.__dict__ == self.__dict__
        setattr(cls, "__eq__", eq_fn)


def _create_py_property(a_name: str, pulumi_name: str, typ: Any, setter: bool = False):
    """
    Returns a Python property getter that looks up the value using get.
    """
    def getter_fn(self):
        return get(self, a_name)
    getter_fn.__name__ = a_name
    getter_fn.__annotations__ = {"return": typ}
    setattr(getter_fn, _PULUMI_NAME, pulumi_name)

    if setter:
        def setter_fn(self, value):
            return set(self, a_name, value)
        setter_fn.__name__ = a_name
        setter_fn.__annotations__ = {"value": typ}
        return builtins.property(fget=getter_fn, fset=setter_fn)

    return builtins.property(fget=getter_fn)


def _py_properties(cls: type) -> Iterator[Tuple[str, str, builtins.property]]:
    for python_name, v in cls.__dict__.items():
        if isinstance(v, builtins.property):
            prop = cast(builtins.property, v)
            pulumi_name = getattr(prop.fget, _PULUMI_NAME, MISSING)
            if pulumi_name is not MISSING:
                yield (python_name, pulumi_name, prop)


def input_type(cls: Type[T]) -> Type[T]:
    """
    Returns the same class as was passed in, but marked as an input type.
    """

    if is_input_type(cls) or is_output_type(cls):
        raise AssertionError("Cannot apply @input_type and @output_type more than once.")

    # Get the input properties and mark the class as an input type.
    _process_class(cls, _PULUMI_INPUT_TYPE, is_input=True, setter=True)

    # Helper to create a setter function.
    def create_setter(name: str) -> Callable:
        def setter_fn(self, value):
            set(self, name, value)
        return setter_fn

    # Now, process the class's properties, replacing properties with empty setters with
    # an actual setter.
    for python_name, _, prop in _py_properties(cls):
        if prop.fset is not None and _utils.is_empty_function(prop.fset):
            setter_fn = create_setter(python_name)
            setter_fn.__name__ = prop.fset.__name__
            setter_fn.__annotations__ = prop.fset.__annotations__
            # Replace the property with a new property object that has the new setter.
            setattr(cls, python_name, prop.setter(setter_fn))

    return cls


def input_type_py_to_pulumi_names(input_type_cls: Type) -> Dict[str, str]:
    """
    Returns a dict of Python names to Pulumi names for the input type.
    """
    assert is_input_type(input_type_cls)
    return {python_name: pulumi_name for python_name, pulumi_name, _ in _py_properties(input_type_cls)}


def input_type_types(input_type_cls: type) -> Dict[str, type]:
    """
    Returns a dict of Pulumi names to types for the input type.
    """
    assert is_input_type(input_type_cls)
    return _types_from_py_properties(input_type_cls)


def input_type_to_dict(obj: Any) -> Dict[str, Any]:
    """
    Returns a dict for the input type.

    The keys of the dict are Pulumi names that should not be translated.
    """
    cls = type(obj)
    assert is_input_type(cls)

    # Build a dictionary of properties to return
    result: Dict[str, Any] = {}
    for _, pulumi_name, prop in _py_properties(cls):
        value = prop.fget(obj)  # type: ignore
        # We treat properties with a value of None as if they don't exist.
        if value is not None:
            result[pulumi_name] = value
    return result


def input_type_to_untranslated_dict(obj: Any) -> Dict[str, Any]:
    """
    Returns an untranslated dict for the input type.
    """
    cls = type(obj)
    assert is_input_type(cls)
    return obj if issubclass(cls, dict) else obj.__dict__


def output_type(cls: Type[T]) -> Type[T]:
    """
    Returns the same class as was passed in, but marked as an output type.

    Python property getters are created for each Pulumi output property
    defined in the class.

    If the class is not a subclass of dict and doesn't have an __init__
    method, an __init__ method is added to the class that accepts a dict
    representing the outputs.
    """

    if is_input_type(cls) or is_output_type(cls):
        raise AssertionError("Cannot apply @input_type and @output_type more than once.")

    # Get the output properties and mark the class as an output type.
    _process_class(cls, _PULUMI_OUTPUT_TYPE)

    # If the class has a _translate_property() method, build a mapping table of Python names to
    # Pulumi names. Calls to pulumi.get() will then convert the name passed to pulumi.get() from
    # the Python name to the Pulumi name, and then pass the Pulumi name to _translate_property() to
    # convert the Pulumi name to whatever name _translate_property() returns (which, for our
    # provider codegen, will be the translated name from _tables.CAMEL_TO_SNAKE_CASE_TABLE).
    # pylint: disable=too-many-nested-blocks
    if hasattr(cls, _TRANSLATE_PROPERTY):
        python_to_pulumi_table = None
        for python_name, pulumi_name, _ in _py_properties(cls):
            if python_name != pulumi_name:
                python_to_pulumi_table = python_to_pulumi_table or {}
                python_to_pulumi_table[python_name] = pulumi_name
        if python_to_pulumi_table is not None:
            setattr(cls, _PULUMI_PYTHON_TO_PULUMI_TABLE, python_to_pulumi_table)

    return cls


def output_type_from_dict(cls: Type[T], output: Dict[str, Any]) -> T:
    assert isinstance(output, dict)
    assert is_output_type(cls)
    args = {}
    for python_name, pulumi_name, _ in _py_properties(cls):
        args[python_name] = output.get(pulumi_name)
    return cls(**args)  # type: ignore


def getter(_fn=None, *, name: Optional[str] = None):
    """
    Decorator to indicate a function is a Pulumi property getter.

    name is the Pulumi property name. If not set, the name of the function is used.
    """
    def decorator(fn: Callable) -> Callable:
        if not callable(fn):
            raise TypeError("Expected fn to be callable")

        # If name isn't specified, use the name of the function.
        pulumi_name = name if name is not None else fn.__name__
        if _utils.is_empty_function(fn):
            @functools.wraps(fn)
            def get_fn(self):
                # Get the value using the Python name, which is the name of the function.
                return get(self, fn.__name__)
            fn = get_fn
        setattr(fn, _PULUMI_NAME, pulumi_name)
        return fn

    # See if we're being called as @getter or @getter().
    if _fn is None:
        # We're called with parens.
        return decorator

    # We're called as @getter without parens.
    return decorator(_fn)


def _translate_name(obj: Any, name: str) -> str:
    cls = type(obj)
    if hasattr(cls, _PULUMI_OUTPUT_TYPE):
        # If the class has a _translate_property() method we need to do two translations:
        #   1. Translate Python => Pulumi name.
        #   2. Translate Pulumi name => result of _translate_property().
        translate = getattr(cls, _TRANSLATE_PROPERTY, None)
        if callable(translate):
            table = getattr(cls, _PULUMI_PYTHON_TO_PULUMI_TABLE, None)
            if isinstance(table, dict):
                name = table.get(name) or name
            name = translate(obj, name)

    return name


def get(self, name: str) -> Any:
    """
    Used to get values in types decorated with @input_type or @output_type.
    """

    if not name:
        raise TypeError("Missing name argument")
    if not isinstance(name, str):
        raise TypeError("Expected name to be a string")

    cls = type(self)

    if hasattr(cls, _PULUMI_INPUT_TYPE) or hasattr(cls, _PULUMI_OUTPUT_TYPE):
        if hasattr(cls, _PULUMI_OUTPUT_TYPE):
            name = _translate_name(self, name)
        if issubclass(cls, dict):
            # Grab dict's `get` method instead of calling `self.get` directly
            # in case the type has a `get` property.
            return getattr(dict, "get")(self, name)
        return self.__dict__.get(name)

    # pylint: disable=import-outside-toplevel
    from . import Resource
    if isinstance(self, Resource):
        return self.__dict__.get(name)

    raise AssertionError("get can only be used with classes decorated with @input_type or @output_type")


def set(self, name: str, value: Any) -> None:
    """
    Used to set values in types decorated with @input_type or @output_type.
    """

    if not name:
        raise TypeError("Missing name argument")
    if not isinstance(name, str):
        raise TypeError("Expected name to be a string")

    cls = type(self)

    if hasattr(cls, _PULUMI_INPUT_TYPE) or hasattr(cls, _PULUMI_OUTPUT_TYPE):
        if hasattr(cls, _PULUMI_OUTPUT_TYPE):
            name = _translate_name(self, name)
        if issubclass(cls, dict):
            self[name] = value
        else:
            self.__dict__[name] = value
        return

    raise AssertionError("set can only be used with classes decorated with @input_type or @output_type")


# Use the built-in `get_origin` and `get_args` functions on Python 3.8+,
# otherwise fallback to downlevel implementations.
if sys.version_info[:2] >= (3, 8):
    # pylint: disable=no-member
    get_origin = typing.get_origin  # type: ignore
    # pylint: disable=no-member
    get_args = typing.get_args  # type: ignore
elif sys.version_info[:2] >= (3, 7):
    def get_origin(tp):
        if isinstance(tp, typing._GenericAlias):  # type: ignore
            return tp.__origin__
        return None

    def get_args(tp):
        if isinstance(tp, typing._GenericAlias):  # type: ignore
            return tp.__args__
        return ()
else:
    def get_origin(tp):
        if hasattr(tp, "__origin__"):
            return tp.__origin__
        return None

    def get_args(tp):
        if hasattr(tp, "__args__"):
            return tp.__args__
        return ()


def _is_union_type(tp):
    if sys.version_info[:2] >= (3, 7):
        return (tp is Union or
                isinstance(tp, typing._GenericAlias) and tp.__origin__ is Union)  # type: ignore
    # pylint: disable=unidiomatic-typecheck, no-member
    return type(tp) is typing._Union  # type: ignore


def _is_optional_type(tp):
    if tp is type(None):
        return True
    if _is_union_type(tp):
        return any(_is_optional_type(tt) for tt in get_args(tp))
    return False


def _types_from_py_properties(cls: type) -> Dict[str, type]:
    """
    Returns a dict of Pulumi names to types for a type.
    """
    # pylint: disable=import-outside-toplevel
    from . import Output

    # We use get_type_hints() below on each Python property to resolve the getter function's
    # return type annotation, resolving forward references.
    #
    # We pass the cls's globals to get_type_hints() to ensure any other referenced
    # output types (which may exist in other modules of the cls, like `.outputs` or
    # `...meta.v1.outputs`) can be resolved. If we didn't pass the cls's globals,
    # get_type_hints() would use the __globals__ of the function, which likely does not contain
    # the necessary references, as the function was likely created internally inside this module
    # (either via the @output_type decorator, which converts class annotations into Python
    # properties, or via the @getter decorator, which replaces empty getter functions) and
    # therefore has __globals__ of this SDK module.
    globalns = None
    if cls.__module__ in sys.modules:
        globalns = dict(sys.modules[cls.__module__].__dict__)

    # Pass along Output as a local, as it is a forward reference type annotation on the base
    # CustomResource class that can be instantiated directly (which our tests do).
    # Additionally, include the `T` TypeVar, which is needed for Input and InputType.
    localns = {"Output": Output, "T": TypeVar("T")}  # type: ignore

    # Build-up a dictionary of Pulumi property names to types by looping through all the
    # Python properties on the class that have a getter marked as a Pulumi property getter,
    # and looking at the getter function's return type annotation.
    # Types that are Output[T] and Optional[T] are unwrapped to just T.
    result: Dict[str, type] = {}
    for _, pulumi_name, prop in _py_properties(cls):
        cls_hints = get_type_hints(prop.fget, globalns=globalns, localns=localns)
        # Get the function's return type hint.
        return_hint = cls_hints.get("return")
        if return_hint is not None:
            typ = unwrap_type(return_hint)
            # If typ is Output, it was specified non-generically (as Output rather than Output[T]),
            # because unwrap_type would have returned the T in Output[T] if it was specified
            # generically. To avoid raising a type mismatch error when the deserialized output type
            # doesn't match Output, we exclude it from the results.
            if typ is Output:
                continue
            result[pulumi_name] = typ
    return result


def _pulumi_to_py_names_from_py_properties(cls: type) -> Dict[str, str]:
    return {
        pulumi_name: python_name
        for python_name, pulumi_name, _ in _py_properties(cls)
    }


def _py_to_pulumi_names_from_py_properties(cls: type) -> Dict[str, str]:
    return {
        python_name: pulumi_name
        for python_name, pulumi_name, _ in _py_properties(cls)
    }


def _types_from_annotations(cls: type) -> Dict[str, type]:
    """
    Returns a dict of Pulumi names to types for a type.
    """
    # Get the "Pulumi properties" from the class's type annotations.
    props = _properties_from_annotations(cls)
    if not props:
        return {}

    # pylint: disable=import-outside-toplevel
    from . import Output

    # We want resolved types for just the cls's type annotations (not base classes),
    # but get_type_hints() looks at the annotations of the class and its base classes.
    # So create a type dynamically that has the annotations from cls but doesn't have
    # any base classes, and pass the dynamically created type to get_type_hints().
    dynamic_cls_attrs = {"__annotations__": cls.__dict__.get("__annotations__", {})}
    dynamic_cls = type(cls.__name__, (object,), dynamic_cls_attrs)

    # Pass along globals for the cls, to help resolve forward references.
    globalns = None
    if getattr(cls, "__module__", None) in sys.modules:
        globalns = dict(sys.modules[cls.__module__].__dict__)

    # Pass along Output as a local, as it is a forward reference type annotation on the base
    # CustomResource class that can be instantiated directly (which our tests do).
    # Additionally, include the `T` TypeVar, which is needed for Input and InputType.
    localns = {"Output": Output, "T": TypeVar("T")}  # type: ignore

    # Get the type hints, resolving any forward references.
    cls_hints = get_type_hints(dynamic_cls, globalns=globalns, localns=localns)

    # Return a dictionary of Pulumi property names to types. Types that are Output[T] and
    # Optional[T] are unwrapped to just T.
    result: Dict[str, type] = {}
    for name, prop in props.items():
        typ = unwrap_type(cls_hints[name])
        # If typ is Output, it was specified non-generically (as Output rather than Output[T]),
        # because unwrap_type would have returned the T in Output[T] if it was specified
        # generically. To avoid raising a type mismatch error when the deserialized output type
        # doesn't match Output, we exclude it from the results.
        if typ is Output:
            continue
        result[prop.name] = typ
    return result


def _names_from_annotations(cls: type) -> Iterator[Tuple[str, str]]:
    # Get annotations that are defined on this class (not base classes).
    # These are returned in the order declared on Python 3.6+.
    cls_annotations = cls.__dict__.get('__annotations__', {})

    def get_pulumi_name(a_name: str) -> str:
        default = getattr(cls, a_name, MISSING)
        return default.name if isinstance(default, _Property) else a_name

    for python_name in cls_annotations.keys():
        yield (python_name, get_pulumi_name(python_name))


def _pulumi_to_py_names_from_annotations(cls: type) -> Dict[str, str]:
    return {
        pulumi_name: python_name
        for python_name, pulumi_name in _names_from_annotations(cls)
    }


def _py_to_pulumi_names_from_annotations(cls: type) -> Dict[str, str]:
    return dict(_names_from_annotations(cls))


def output_type_types(output_type_cls: type) -> Dict[str, type]:
    """
    Returns a dict of Pulumi names to types for the output type.
    """
    assert is_output_type(output_type_cls)
    return _types_from_py_properties(output_type_cls)


def resource_types(resource_cls: type) -> Dict[str, type]:
    """
    Returns a dict of Pulumi names to types for the resource.
    """
    # First, get the types from the class's type annotations.
    types_from_annotations = _types_from_annotations(resource_cls)

    # Next, get the types from the class's Python properties.
    types_from_py_properties = _types_from_py_properties(resource_cls)

    # Return the merged dictionaries.
    return {**types_from_annotations, **types_from_py_properties}


def resource_pulumi_to_py_names(resource_cls: type) -> Dict[str, str]:
    """
    Returns a dict of Pulumi names to types for the resource.
    """
    # First, get the names from the class's type annotations.
    names_from_annotations = _pulumi_to_py_names_from_annotations(resource_cls)

    # Next, get the names from the class's Python properties.
    names_from_py_properties = _pulumi_to_py_names_from_py_properties(resource_cls)

    # Return the merged dictionaries.
    return {**names_from_annotations, **names_from_py_properties}


def resource_py_to_pulumi_names(resource_cls: type) -> Dict[str, str]:
    """
    Returns a dict of Pulumi names to types for the resource.
    """
    # First, get the names from the class's type annotations.
    names_from_annotations = _py_to_pulumi_names_from_annotations(resource_cls)

    # Next, get the names from the class's Python properties.
    names_from_py_properties = _py_to_pulumi_names_from_py_properties(resource_cls)

    # Return the merged dictionaries.
    return {**names_from_annotations, **names_from_py_properties}


def unwrap_optional_type(val: type) -> type:
    """
    Unwraps the type T in Optional[T].
    """
    # If it is Optional[T], extract the arg T. Note that Optional[T] is really Union[T, None],
    # and any nested Unions are flattened, so Optional[Union[T, U], None] is Union[T, U, None].
    # We'll only "unwrap" for the common case of a single arg T for Union[T, None].
    if _is_optional_type(val):
        args = get_args(val)
        if len(args) == 2:
            assert args[1] is type(None)
            val = args[0]

    return val


def unwrap_type(val: type) -> type:
    """
    Unwraps the type T in Output[T], Input[T], InputType[T], and Optional[T].
    """
    # pylint: disable=import-outside-toplevel
    from . import Output

    origin = get_origin(val)

    # If it is an Output[T], extract the T arg.
    if origin is Output:
        args = get_args(val)
        assert len(args) == 1
        val = args[0]

    # If it looks like Input[T] or InputType[T], extract the T arg.
    if _is_union_type(val):
        def isInputType(args):
            assert len(args) > 1
            return (is_input_type(args[0]) and
                args[1] is dict or get_origin(args[1]) in {dict, Dict, Mapping, collections.abc.Mapping})

        def isInput(args, i = 1):
            assert len(args) > i + 1
            return get_origin(args[i]) is collections.abc.Awaitable and get_origin(args[i + 1]) is Output

        args = get_args(val)
        if len(args) == 2:
            if isInputType(args): # InputType[T]
                return args[0]
        elif len(args) == 3:
            if isInput(args): # Input[T]
                return args[0]
            if isInputType(args) and args[2] is type(None): # Optiona[InputType[T]]
                return args[0]
        elif len(args) == 4:
            if isInput(args) and args[3] is type(None): # Optional[Input[T]]
                return args[0]
            if isInputType(args) and isInput(args, 2): # Input[InputType[T]]
                return args[0]
        elif len(args) == 5:
            if isInputType(args) and isInput(args, 2) and args[4] is type(None): # Optional[Input[InputType[T]]]
                return args[0]

    return unwrap_optional_type(val)


# The following functions for creating an __init__() method were adapted
# from Python's dataclasses module.

def _create_fn(name, args, body, *, globals=None, locals=None):
    if locals is None:
        locals = {}
    if "BUILTINS" not in locals:
        locals["BUILTINS"] = builtins
    args = ",".join(args)
    body = "\n".join(f"  {b}" for b in body)

    # Compute the text of the entire function.
    txt = f" def {name}({args}):\n{body}"

    local_vars = ", ".join(locals.keys())
    txt = f"def __create_fn__({local_vars}):\n{txt}\n return {name}"

    ns = {}
    exec(txt, globals, ns)  # pylint: disable=exec-used
    return ns["__create_fn__"](**locals)


def _property_init(python_name: str, prop: _Property, globals, is_dict: bool, has_translate: bool):
    # Return the text of the line in the body of __init__() that will
    # initialize this property.

    default_name = f"_dflt_{python_name}"
    if prop.default is MISSING:
        # There's no default, just do an assignment.
        value = python_name
    else:
        globals[default_name] = python_name
        value = python_name

    # Now, actually generate the assignment.
    if is_dict:
        # It's a dict, store the value in itself.
        container = ""
    else:
        # It isn't a dict, store the value in __dict__.
        container = ".__dict__"

    # Only assign the value if not None.
    if prop.default is None:
        check = f"if {value} is not None:\n    "
    else:
        check = ""

    # If it has a _translate_property method, use it to translate the name.
    if has_translate:
        return f"{check}__self__{container}[__self__.{_TRANSLATE_PROPERTY}('{prop.name}')]={value}"

    return f"{check}__self__{container}['{python_name}']={value}"


def _init_param(python_name: str, prop: _Property):
    # Return the __init__ parameter string for this property.  For
    # example, the equivalent of 'x:int=3' (except instead of 'int',
    # reference a variable set to int, and instead of '3', reference a
    # variable set to 3).
    if prop.default is MISSING:
        # There's no default, just output the variable name and type.
        default = ""
    else:
        # There's a default, this will be the name that's used to look it up.
        default = f"=_dflt_{python_name}"
    return f"{python_name}:_type_{python_name}{default}"


def _init_fn(props: Dict[str, _Property], globals, is_dict: bool, has_translate: bool):
    # Make sure we don't have properties without defaults following properties
    # with defaults. This actually would be caught when exec-ing the
    # function source code, but catching it here gives a better error
    # message, and future-proofs us in case we build up the function
    # using ast.
    seen_default = False
    for python_name, prop in props.items():
        if prop.default is not MISSING:
            seen_default = True
        elif seen_default:
            raise TypeError(f"non-default argument {python_name!r} "
                            "follows default argument")

    locals = {f"_type_{python_name}": prop.type for python_name, prop in props.items()}
    locals.update({
        "MISSING": MISSING,
    })

    body_lines = []
    for python_name, prop in props.items():
        line = _property_init(python_name, prop, locals, is_dict, has_translate)
        body_lines.append(line)

    # If no body lines, use `pass`.
    if not body_lines:
        body_lines = ["pass"]

    first_args = ["__self__"]
    # If we have args after __self__, use bare * to force them to be specified by name.
    if len(props) > 0:
        first_args += ["*"]

    return _create_fn("__init__",
                      first_args + [_init_param(python_name, prop) for python_name, prop in props.items()],
                      body_lines,
                      locals=locals,
                      globals=globals)
