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


from contextvars import ContextVar
import importlib
import importlib.abc
import sys
import types
import typing


# Empty function definitions.


def _empty(): ...


def _empty_doc():
    """Empty function docstring."""


def _empty_lambda():
    return None


def _empty_lambda_doc():
    return None


_empty_lambda_doc.__doc__ = """Empty lambda docstring."""


def _consts(fn: typing.Callable) -> tuple:
    """
    Returns a tuple of the function's constants excluding the docstring.
    """
    return tuple(x for x in fn.__code__.co_consts if x != fn.__doc__)


# Precompute constants for each of the empty functions.
_consts_empty = _consts(_empty)
_consts_empty_doc = _consts(_empty_doc)
_consts_empty_lambda = _consts(_empty_lambda)
_consts_empty_lambda_doc = _consts(_empty_lambda_doc)


def is_empty_function(fn: typing.Callable) -> bool:
    """
    Returns true if the function is empty.
    """
    consts = _consts(fn)
    return (
        (fn.__code__.co_code == _empty.__code__.co_code and consts == _consts_empty)
        or (
            fn.__code__.co_code == _empty_doc.__code__.co_code
            and consts == _consts_empty_doc
        )
        or (
            fn.__code__.co_code == _empty_lambda.__code__.co_code
            and consts == _consts_empty_lambda
        )
        or (
            fn.__code__.co_code == _empty_lambda_doc.__code__.co_code
            and consts == _consts_empty_lambda_doc
        )
    )


def lazy_import(fullname):
    """Defers module import until first attribute access. For example:

    import a.b.c as x

    Becomes:

    x = lazy_import('a.b.c')

    The code started from the official Python example:

    https://github.com/python/cpython/blob/master/Doc/library/importlib.rst#implementing-lazy-imports

    This example is extended by early returns to support import cycles
    and registration of sub-modules as attributes.
    """

    # Return early if already registered; this supports import cycles.
    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    spec = importlib.util.find_spec(fullname)

    # Return early if find_spec has recursively called lazy_import
    # again and pre-populated the sys.modules slot; an example of this
    # is covered by test_lazy_import.
    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    loader = _LazyLoader(spec.loader)
    spec.loader = loader
    module = importlib.util.module_from_spec(spec)

    # Return early rather than overwriting the sys.modules slot.
    m = sys.modules.get(fullname, None)
    if m is not None:
        return m

    sys.modules[fullname] = module
    loader.exec_module(module)
    return module


def contextproperty(fn=None, *, default: typing.Optional[typing.Any] = None):
    """Decorator interface for ContextProperty

    This gives a @property-like interface into ContextProperty:

    .. testsetup::

        class Foo:
            @contextproperty(default="bar")
            def my_attribute(): ...


    >>> class Foo:
    ...    @contextproperty(default="bar")
    ...    def my_attribute(): ...
    >>> Foo().my_attribute
    'bar'
    """

    def inner(func: typing.Callable):
        new_property = ContextProperty(
            name=func.__qualname__, doc=func.__doc__, default=default
        )
        inner.__doc__ = new_property.__doc__
        return new_property

    if fn is None:
        return inner
    return inner(fn)


# We don't currently use doctest anywhere, but they can be helpful
# usage guides - we double-declare Foo in this case because the testsetup directive
# is hidden from docstrings
class ContextProperty:
    """Property-like interface for ContextVars

    This class is intended to wrap attribute access in class instances in a property-like interface
    while also ensuring the data being stored does not leak between contexts.

    The preferred method for using this class is via the @contextproperty decorator, though it
    can also be used directly:

    .. testsetup::

        class Foo:
            my_attribute = ContextProperty(name="foo", default="bar")


    >>> class Foo:
    ...     my_attribute = ContextProperty(name="foo", default="bar")
    >>> Foo().my_attribute
    'bar'
    """

    def __init__(
        self,
        *_,
        name: str,
        doc: typing.Optional[str] = None,
        default: typing.Optional[typing.Any] = None,
    ):
        """
        :param str name: The name assigned to both the property and also passed to the underlying ContextVar
        :param str doc: Docstring to assign to this property
        :param Any default: Default value to be passed to the underlying ContextVar
        """
        if not doc:
            doc = self.fget.__doc__
        self.__doc__ = doc
        self._name = name
        self.__default = default
        self._data: ContextVar = ContextVar(name, default=default)

    def __repr__(self):
        return f"<class {type(self).__qualname__}[name={self._data.name.__repr__()} default={self.__default.__repr__()}] value: {self._data.get().__repr__()}>"

    def fget(self, *_):
        return self._data.get(self.__default)

    def fset(self, v: typing.Any, *_):
        if self._data.get() is None:
            self.__default = v
        return self._data.set(v)

    def fdel(self, *_):
        return self._data.reset()

    def __set__(self, _, v: typing.Any):
        return self.fset(v)

    def __get__(self, obj: typing.Optional[typing.Any], _):
        if obj is None or isinstance(self.fget(), dict):
            return self
        return self.fget()

    def __delete__(self, *_):
        self.fdel()

    def __setitem__(self, key, value):
        data = self.fget()
        if not isinstance(data, dict):
            raise TypeError("Can't set key values on non-dict variables.")
        data[key] = value
        return self.fset(data)

    def __dict__(self):
        if not isinstance(self.fget(), dict):
            raise TypeError(f"{self} is not a dictionary")
        return self.fget()

    def __getitem__(self, key):
        return self.fget()[key]

    def get(self, key, default=None):
        data = self.fget()
        if not isinstance(data, dict):
            raise TypeError("Can't get key values on non-dict variables.")
        return data.get(key, default)

    def __contains__(self, element):
        return element in self.fget()

    def getter(self, fget):
        prop = type(self)(fget, self.fset, self.fdel, name=self._name, doc=self.__doc__)
        return prop

    def setter(self, fset):
        prop = type(self)(self.fget, fset, self.fdel, name=self._name, doc=self.__doc__)
        return prop

    def deleter(self, fdel):
        prop = type(self)(self.fget, self.fset, fdel, name=self._name, doc=self.__doc__)
        return prop


# Python 3.11.9 and 3.12.3 introduced a regression that broke lazy loading of
# `config` modules in generated provider SDKs.
# See https://github.com/python/cpython/pull/117185#issuecomment-2060851286.
# To workaround, use a private copy of _LazyModule and LazyLoader (renamed
# _LazyLoader) from the Python stdlib, which don't have the regression,
# as suggested by the Python maintainers.
class _LazyModule(types.ModuleType):
    """A subclass of the module type which triggers loading upon attribute access."""

    def __getattribute__(self, attr):
        """Trigger the load of the module and return the attribute."""
        # All module metadata must be garnered from __spec__ in order to avoid
        # using mutated values.
        # Stop triggering this method.
        self.__class__ = types.ModuleType
        # Get the original name to make sure no object substitution occurred
        # in sys.modules.
        original_name = self.__spec__.name
        # Figure out exactly what attributes were mutated between the creation
        # of the module and now.
        attrs_then = self.__spec__.loader_state["__dict__"]
        attrs_now = self.__dict__
        attrs_updated = {}
        for key, value in attrs_now.items():
            # Code that set the attribute may have kept a reference to the
            # assigned object, making identity more important than equality.
            if key not in attrs_then:
                attrs_updated[key] = value
            elif id(attrs_now[key]) != id(attrs_then[key]):
                attrs_updated[key] = value
        self.__spec__.loader.exec_module(self)
        # If exec_module() was used directly there is no guarantee the module
        # object was put into sys.modules.
        if original_name in sys.modules:
            if id(self) != id(sys.modules[original_name]):
                raise ValueError(
                    f"module object for {original_name!r} "
                    "substituted in sys.modules during a lazy "
                    "load"
                )
        # Update after loading since that's what would happen in an eager
        # loading situation.
        self.__dict__.update(attrs_updated)
        return getattr(self, attr)

    def __delattr__(self, attr):
        """Trigger the load and then perform the deletion."""
        # To trigger the load and raise an exception if the attribute
        # doesn't exist.
        self.__getattribute__(attr)
        delattr(self, attr)


class _LazyLoader(importlib.abc.Loader):
    """A loader that creates a module which defers loading until attribute access."""

    @staticmethod
    def __check_eager_loader(loader):
        if not hasattr(loader, "exec_module"):
            raise TypeError("loader must define exec_module()")

    @classmethod
    def factory(cls, loader):
        """Construct a callable which returns the eager loader made lazy."""
        cls.__check_eager_loader(loader)
        return lambda *args, **kwargs: cls(loader(*args, **kwargs))

    def __init__(self, loader):
        self.__check_eager_loader(loader)
        self.loader = loader

    def create_module(self, spec):
        return self.loader.create_module(spec)

    def exec_module(self, module):
        """Make the module load lazily."""
        module.__spec__.loader = self.loader
        module.__loader__ = self.loader
        # Don't need to worry about deep-copying as trying to set an attribute
        # on an object would have triggered the load,
        # e.g. ``module.__spec__.loader = None`` would trigger a load from
        # trying to access module.__spec__.
        loader_state = {}
        loader_state["__dict__"] = module.__dict__.copy()
        loader_state["__class__"] = module.__class__
        module.__spec__.loader_state = loader_state
        module.__class__ = _LazyModule
