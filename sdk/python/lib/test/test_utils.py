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

import sys
import os.path

import unittest

from pulumi._utils import is_empty_function, lazy_import, _LazyModule, _INTROSPECTION_ATTRS


# Function with return value based on input, called in the non_empty function
# bodies below.
def compute(val: int) -> str:
    return f"{val} + {1} = {val + 1}"


class Foo:
    def empty_a(self) -> str: ...  # type: ignore

    def empty_b(self) -> str:  # type: ignore
        """A docstring."""
        ...

    def empty_c(self, value: str): ...

    def non_empty_a(self) -> str:
        return "hello"

    def non_empty_b(self) -> str:
        """A docstring."""
        return "hello"

    def non_empty_c(self) -> str:
        return compute(41)

    def non_empty_d(self) -> str:
        """F's docstring."""
        return compute(41)


empty_lambda_a = lambda: None

empty_lambda_b = lambda: None
empty_lambda_b.__doc__ = """A docstring."""

non_empty_lambda_a = lambda: "hello"

non_empty_lambda_b = lambda: "hello"
non_empty_lambda_b.__doc__ = """A docstring."""

non_empty_lambda_c = lambda: compute(41)

non_empty_lambda_d = lambda: compute(41)
non_empty_lambda_d.__doc__ = """A docstring."""


class IsEmptyFunctionTests(unittest.TestCase):
    def test_is_empty(self):
        f = Foo()

        self.assertTrue(is_empty_function(Foo.empty_a))
        self.assertTrue(is_empty_function(Foo.empty_b))
        self.assertTrue(is_empty_function(Foo.empty_c))
        self.assertTrue(is_empty_function(f.empty_a))
        self.assertTrue(is_empty_function(f.empty_b))
        self.assertTrue(is_empty_function(f.empty_c))

        self.assertFalse(is_empty_function(Foo.non_empty_a))
        self.assertFalse(is_empty_function(Foo.non_empty_b))
        self.assertFalse(is_empty_function(Foo.non_empty_c))
        self.assertFalse(is_empty_function(Foo.non_empty_d))
        self.assertFalse(is_empty_function(f.non_empty_a))
        self.assertFalse(is_empty_function(f.non_empty_b))
        self.assertFalse(is_empty_function(f.non_empty_c))
        self.assertFalse(is_empty_function(f.non_empty_d))

        self.assertTrue(is_empty_function(empty_lambda_a))
        self.assertTrue(is_empty_function(empty_lambda_b))

        self.assertFalse(is_empty_function(non_empty_lambda_a))
        self.assertFalse(is_empty_function(non_empty_lambda_b))
        self.assertFalse(is_empty_function(non_empty_lambda_c))
        self.assertFalse(is_empty_function(non_empty_lambda_d))


def test_lazy_import():
    sys.path.append(os.path.join(os.path.dirname(__file__), "data"))
    x = lazy_import("lazy_import_test.x")
    y = lazy_import("lazy_import_test.y")
    test = lazy_import("lazy_import_test")

    assert test.x.foo() == "foo"
    assert x.foo() == "foo"
    assert id(x) == id(test.x)

    assert test.y.foo == "foo"
    assert y.foo == "foo"
    assert id(y) == id(test.y)


def test_lazy_module_introspection_attrs_do_not_trigger_load():
    """Test that accessing introspection attributes doesn't trigger module load.

    This is important for tools like debuggers, file watchers, IDEs, etc.
    that iterate over sys.modules and check module attributes.
    See https://github.com/streamlit/streamlit/issues/13530
    """
    sys.path.append(os.path.join(os.path.dirname(__file__), "data"))

    # Clear any cached module to ensure fresh lazy load
    for mod_name in list(sys.modules.keys()):
        if mod_name.startswith("lazy_import_test"):
            del sys.modules[mod_name]

    # Create a lazy module
    module = lazy_import("lazy_import_test.x")

    # Verify it starts as a _LazyModule
    assert module.__class__ == _LazyModule, "Module should start as _LazyModule"

    # Access introspection attributes - these should NOT trigger loading
    for attr in _INTROSPECTION_ATTRS:
        try:
            getattr(module, attr)
        except AttributeError:
            # Some attributes may not exist, that's fine
            pass

    # Module should still be lazy (not loaded) after accessing introspection attrs
    assert module.__class__ == _LazyModule, (
        f"Module should still be _LazyModule after accessing introspection attrs, "
        f"but was {module.__class__}"
    )

    # Now access a real attribute - this should trigger loading
    result = module.foo()
    assert result == "foo"

    # Module should now be fully loaded
    assert module.__class__ == type(sys), "Module should now be fully loaded"
