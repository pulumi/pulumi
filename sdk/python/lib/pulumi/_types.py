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
#
# Here's how the `NestedArgs` input class can be declared:
#
#   @pulumi.input_type
#   class NestedArgs:
#       first_arg: pulumi.Input[str] = pulumi.property("firstArg")
#       second_arg: Optional[pulumi.Input[float]] = pulumi.property("secondArg")
#
#       def __init__(self, first_arg: pulumi.Input[str], second_arg: Optional[pulumi.Input[float]] = None):
#           pulumi.set(self, "firstArg", first_arg)
#           pulumi.set(self, "secondArg", second_arg)
#
# The class is decorated with the `@pulumi.input_type` decorator, which indicates the class is an
# input type and does some processing of the class (explained below). `NestedArgs` declares two
# inputs (`first_arg` and `second_arg`) and uses type annotations and `pulumi.property()` to
# specify the types and Pulumi input property names. The `__init__()` method class `pulumi.set` to
# save the values for each input.
#
# A more verbose way to declare the same input type is as follows:
#
#   @pulumi.input_type
#   class NestedArgs:
#       def __init__(self, first_arg: pulumi.Input[str], second_arg: Optional[pulumi.Input[float]] = None):
#           pulumi.set(self, "firstArg", first_arg)
#           pulumi.set(self, "secondArg", second_arg)
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
# the `@pulumi.input_type` processes the class and essentially transforms the former declaration
# into the latter declaration.
#
# The former (simpler) declaration is nice syntactic sugar to use when declaring these by hand,
# e.g. when writing a dynamic provider that has nested inputs/outputs. The latter declaration isn't
# as pleasant to write by hand and is closer to what we emit in our provider codegen. The benefit
# of the latter (more verbose) form is that it allows docstrings to be specified on the Python
# property getters, which will show up in IDE tooltips when hovering over the property.
#
# Note the property getter/setter functions are empty in the latter (more verbose) declaration.
# Empty getter functions are automatically replaced by the `@pulumi.getter` decorator with an
# actual implementation, and the `@pulumi.input_type` decorator will automatically replace any
# empty setter functions associated with a getter decorated with `@pulumi.getter` with an actual
# implementation. Thus, the above is equivalent to this even more verbose form:
#
#   @pulumi.input_type
#   class NestedArgs:
#       def __init__(self, first_arg: pulumi.Input[str], second_arg: Optional[pulumi.Input[float]] = None):
#           pulumi.set(self, "firstArg", first_arg)
#           pulumi.set(self, "secondArg", second_arg)
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> pulumi.Input[str]:
#           return pulumi.get(self, "firstArg")
#
#       @first_arg.setter
#       def first_arg(self, value: pulumi.Input[str]):
#           pulumi.set(self, "firstArg", value)
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[pulumi.Input[float]]:
#           return pulumi.get(self, "secondArg")
#
#       @second_arg.setter
#       def second_arg(self, value: Optional[pulumi.Input[float]]):
#           pulumi.set(self, "secondArg", value)
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
#       def __init__(self, values: Dict[str, Any]):
#           self._values = values
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
# `__init__()` method isn't already present on the class) which accepts a dictionary containing the
# values of the object. When the output is resolved, the `Nested` class is instantiated and the
# dict representing the object from the engine is passed to class's `__init__()` method.
#
# Output types only have property getters and the bodies can be empty. Empty getter functions are
# replaced with implementations by the `@pulumi.getter` decorator.
#
# The above form is equivalent to:
#
#   @pulumi.output_type
#   class Nested:
#       def __init__(self, values: Dict[str, Any]):
#           self._values = values
#
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> str:
#           return pulumi.get(self, "firstArg")
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[float]:
#           return pulumi.get(self, "secondArg")
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
#       @property
#       @pulumi.getter(name="firstArg")
#       def first_arg(self) -> str:
#           return pulumi.get(self, "firstArg")
#
#       @property
#       @pulumi.getter(name="secondArg")
#       def second_arg(self) -> Optional[float]:
#           return pulumi.get(self, "secondArg")
#
#
# Note: When the class is a subclass of `dict`, no `__init__()` method is generated, as super
# type's (dict's) `__init__()` will be used and calls to `pulumi.get` will get the values from
# itself.
#
# An output class can optionally include a `_translate_property(self, prop)` method, which
# `pulumi.get` will call to translate the Pulumi property name to a translated key name before
# looking up the value in the dictionary. This is to provide backwards compatibility with our
# provider generated code, where mapping tables are used to translate dict keys before being
# returned to the program. This way, existing programs accessing the values as a dictionary will
# continue to see the same translated key names as before, but updated programs can now access
# the values using Python properties, which will always have thecorrect snake_case Python names.
#
#   @pulumi.output_type
#   class Nested(dict):
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
import functools
import sys
import typing
from typing import Any, Callable, Dict, Optional, Type, TypeVar, Union, cast, get_type_hints

from . import _utils

T = TypeVar('T')


_PULUMI_GETTER = "_pulumi_getter"
_PULUMI_NAME = "_pulumi_name"
_PULUMI_INPUT_TYPE = "_pulumi_input_type"
_PULUMI_OUTPUT_TYPE = "_pulumi_output_type"
_TRANSLATE_PROPERTY = "_translate_property"
_VALUES = "_values"


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
def property(name: str, default: Any = MISSING) -> Any:
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


def _process_class(cls: type, signifier_attr: str) -> Dict[str, Any]:
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

    return props


def _create_py_property(a_name: str, pulumi_name: str, typ: Any, setter: bool):
    """
    Returns a Python property getter that looks up the value using get.
    """
    def getter_fn(self):
        return get(self, pulumi_name)
    getter_fn.__name__ = a_name
    getter_fn.__annotations__ = {"return":typ}
    setattr(getter_fn, _PULUMI_GETTER, True)
    setattr(getter_fn, _PULUMI_NAME, pulumi_name)

    if setter:
        def setter_fn(self, value):
            return set(self, pulumi_name, value)
        setter_fn.__name__ = a_name
        setter_fn.__annotations__ = {"value":typ}
        return builtins.property(fget=getter_fn, fset=setter_fn)

    return builtins.property(fget=getter_fn)


def _add_eq(cls: type):
    # Add an __eq__ method to cls if it isn't a subclass of dict and __eq__ doesn't already exist.
    # There's no need for a __ne__ method, since Python will call __eq__ and negate it.
    if not issubclass(cls, dict) and "__eq__" not in cls.__dict__:
        def eq(self, other):
            return type(other) is type(self) and getattr(other, _VALUES, None) == getattr(self, _VALUES, None)
        setattr(cls, "__eq__", eq)


def input_type(cls: Type[T]) -> Type[T]:
    """
    Returns the same class as was passed in, but marked as an input type.
    """

    if is_input_type(cls) or is_output_type(cls):
        raise AssertionError("Cannot apply @input_type and @output_type more than once.")

    # Get the input properties and mark the class as an input type.
    props = _process_class(cls, _PULUMI_INPUT_TYPE)

    # Create Python properties.
    for name, prop in props.items():
        setattr(cls, name, _create_py_property(name, prop.name, prop.type, setter=True))

    # Helper to create a setter function.
    def create_setter(pulumi_name: str) -> Callable:
        def setter_fn(self, value):
            set(self, pulumi_name, value)
        return setter_fn

    # Now, process the class's properties, replacing properties with empty setters with
    # an actual setter.
    for k, v in cls.__dict__.items():
        if isinstance(v, builtins.property):
            prop = cast(builtins.property, v)
            if hasattr(prop.fget, _PULUMI_GETTER) and prop.fset is not None and _utils.is_empty_function(prop.fset):
                pulumi_name: str = getattr(prop.fget, _PULUMI_NAME)
                setter_fn = create_setter(pulumi_name)
                setter_fn.__name__ = prop.fset.__name__
                setter_fn.__annotations__ = prop.fset.__annotations__
                # Replace the property with a new property object that has the new setter.
                setattr(cls, k, prop.setter(setter_fn))

    # Add an __eq__ method if one doesn't already exist.
    _add_eq(cls)

    return cls


def input_type_to_dict(value: Any) -> Dict[str, Any]:
    """
    Returns a dict for the input type.
    """
    assert is_input_type(type(value))
    return dict(getattr(value, _VALUES, {}))


def output_type(cls: Type[T]) -> Type[T]:
    """
    Returns the same class as was passed in, but marked as an output type.

    Python property getters are created for each Pulumi output property
    defined in the class.

    If the class is not a subclass of dict and doesn't have an __init__()
    method, an __init__() method is added to the class that accepts a dict
    representing the outputs.
    """

    if is_input_type(cls) or is_output_type(cls):
        raise AssertionError("Cannot apply @input_type and @output_type more than once.")

    # Get the output properties and mark the class as an output type.
    props = _process_class(cls, _PULUMI_OUTPUT_TYPE)

    # Add an __init__() method that takes a dict (representing outputs) as an arg,
    # if the class isn't a subclass of dict and doesn't have an __init__() method.
    if not issubclass(cls, dict) and "__init__" not in cls.__dict__:
        def init(self, value: dict) -> None:
            if not isinstance(value, dict):
                raise TypeError('Expected value to be a dict')
            setattr(self, _VALUES, value)
        setattr(cls, "__init__", init)

    # Create Python properties.
    for name, prop in props.items():
        setattr(cls, name, _create_py_property(name, prop.name, prop.type, setter=False))

    # Add an __eq__ method if one doesn't already exist.
    _add_eq(cls)

    return cls


def getter(_fn=None, *, name: Optional[str] = None):
    """
    Decorator to indicate a function is a Pulumi property getter.

    name is the Pulumi property name. If not set, the name of the function is used.
    """
    def decorator(fn: Callable) -> Callable:
        # If name isn't specified, use the name of the function.
        pulumi_name = name if name is not None else fn.__name__
        if _utils.is_empty_function(fn):
            @functools.wraps(fn)
            def get_fn(self):
                return get(self, pulumi_name)
            fn = get_fn
        setattr(fn, _PULUMI_GETTER, True)
        setattr(fn, _PULUMI_NAME, pulumi_name)
        return fn

    # See if we're being called as @getter or @getter().
    if _fn is None:
        # We're called with parens.
        return decorator

    # We're called as @getter without parens.
    return decorator(_fn)


def get(self, name: str) -> Any:
    """
    Used to get values in Pulumi property getters.

    name is the Pulumi property name.
    """

    if not name:
        raise TypeError("Missing name argument")
    if not isinstance(name, str):
        raise TypeError("Expected name to be a string")

    values: Optional[Dict[str, Any]] = getattr(self, _VALUES, None)

    if hasattr(type(self), _PULUMI_INPUT_TYPE):
        return values.get(name) if values is not None else None

    if hasattr(type(self), _PULUMI_OUTPUT_TYPE):
        cls = type(self)

        # If the class has a _translate_property() method, use it to translate
        # property names, otherwise, use an identity function.
        translate = getattr(cls, _TRANSLATE_PROPERTY, None)
        if not callable(translate):
            translate = lambda self, prop: prop

        # If the class itself is a subclass of dict, get the value from itself,
        # otherwise, get the value from a private _values attribute.
        if issubclass(cls, dict):
            # Grab dict's `get` method instead of calling `self.get` directly
            # in case the type has a `get` property.
            return getattr(dict, "get")(self, translate(self, name))

        return values.get(translate(self, name)) if values is not None else None

    raise AssertionError("get can only be used with classes decorated with @input_type or @output_type")


def set(self, name: str, value: Any) -> None:
    """
    Used to set values in the __init__() method of classes decorated with @input_type.

    name is the Pulumi property name.
    """

    if not name:
        raise TypeError("Missing name argument")
    if not isinstance(name, str):
        raise TypeError("Expected name to be a string")

    if not hasattr(type(self), _PULUMI_INPUT_TYPE):
        raise AssertionError("set can only be used with classes decorated with @input_type")

    values = getattr(self, _VALUES, None)
    if values is None:
        values = dict()
        setattr(self, _VALUES, values)
    values[name] = value


# Use the built-in `get_origin` and `get_args` functions on Python 3.8+,
# otherwise fallback to downlevel implementations.
if sys.version_info[:2] >= (3, 8):
    get_origin = typing.get_origin  # pylint: disable=no-member
    get_args = typing.get_args  # pylint: disable=no-member
elif sys.version_info[:2] >= (3, 7):
    def get_origin(tp):
        if isinstance(tp, typing._GenericAlias):
            return tp.__origin__
        return None

    def get_args(tp):
        if isinstance(tp, typing._GenericAlias):
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
                isinstance(tp, typing._GenericAlias) and tp.__origin__ is Union)
    return type(tp) is typing._Union # pylint: disable=unidiomatic-typecheck, no-member


def _is_optional_type(tp):
    if tp is type(None):
        return True
    if _is_union_type(tp):
        return any(_is_optional_type(tt) for tt in get_args(tp))
    return False


def output_type_types(output_type_cls: type) -> Dict[str, type]:
    """
    Returns a dict of Pulumi names to type for the output type.
    """
    assert is_output_type(output_type_cls)

    # pylint: disable=import-outside-toplevel
    from . import Output, Input

    result: Dict[str, type] = {}

    for v in output_type_cls.__dict__.values():
        if isinstance(v, builtins.property):
            prop = cast(builtins.property, v)
            if hasattr(prop.fget, _PULUMI_GETTER) and hasattr(prop.fget, _PULUMI_NAME):
                name: str = getattr(prop.fget, _PULUMI_NAME)
                 # Get hints via typing.get_type_hints(), which handles forward references.
                 # Pass Output and Input as locals to typing.get_type_hints() to ensure they are available.
                cls_hints = get_type_hints(prop.fget, localns={"Output": Output, "Input": Input})  # type: ignore
                value = cls_hints.get("return")
                if value is not None:
                    result[name] = _unwrap_type(value)

    return result


def resource_types(resource_cls: type) -> Dict[str, type]:
    """
    Returns a dict of Pulumi names to type for the resource.
    """
    # pylint: disable=import-outside-toplevel
    from . import Output, Input

    props = _properties_from_annotations(resource_cls)

    # Get hints via typing.get_type_hints(), which handles forward references.
    # Pass Output and Input as locals to typing.get_type_hints() to ensure they are available.
    cls_hints = get_type_hints(resource_cls, localns={"Output": Output, "Input": Input})  # type: ignore

    return {
        prop.name: _unwrap_type(cls_hints[name])
        for name, prop in props.items()
    }


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


def _unwrap_type(val: type) -> type:
    """
    Unwraps the type T in Output[T] and Optional[T].
    """
    # pylint: disable=import-outside-toplevel
    from . import Output

    origin = get_origin(val)

    # If it is an Output[T], extract the T arg.
    if origin is Output:
        args = get_args(val)
        assert len(args) == 1
        val = args[0]

    return unwrap_optional_type(val)
