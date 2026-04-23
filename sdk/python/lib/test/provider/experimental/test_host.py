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

import warnings

from pulumi.provider.experimental.host import _validate_explicit_components
from pulumi.resource import ComponentResource, CustomResource


class MyComponent(ComponentResource):
    def __init__(self, name, args=None, opts=None):
        super().__init__("test:index:MyComponent", name, {}, opts)


class MyOtherComponent(ComponentResource):
    def __init__(self, name, args=None, opts=None):
        super().__init__("test:index:MyOtherComponent", name, {}, opts)


class MyCustom(CustomResource):
    pass


class NotAResource:
    pass


def test_no_warning_for_valid_components():
    with warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        result = _validate_explicit_components([MyComponent, MyOtherComponent])
    assert result == [MyComponent, MyOtherComponent]
    assert w == []


def test_no_warning_for_empty_list():
    with warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        result = _validate_explicit_components([])
    assert result == []
    assert w == []


def test_warns_for_customresource_subclass():
    with warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        result = _validate_explicit_components([MyCustom])
    assert result == []
    assert len(w) == 1
    msg = str(w[0].message)
    assert "MyCustom" in msg
    assert "ComponentResource" in msg


def test_warns_for_plain_class():
    with warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        result = _validate_explicit_components([NotAResource])
    assert result == []
    assert len(w) == 1
    assert "NotAResource" in str(w[0].message)


def test_warns_for_non_class_values():
    with warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        result = _validate_explicit_components(["MyComponent", None, 42, lambda: None])  # type: ignore[list-item]
    # All four should warn, none are valid components
    assert result == []
    assert len(w) == 4


def test_keeps_valid_warns_for_invalid_mixed():
    with warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        result = _validate_explicit_components(
            [MyComponent, MyCustom, MyOtherComponent, NotAResource]
        )  # type: ignore[list-item]
    assert result == [MyComponent, MyOtherComponent]
    assert len(w) == 2
    messages = [str(x.message) for x in w]
    assert any("MyCustom" in m for m in messages)
    assert any("NotAResource" in m for m in messages)


def test_subclass_of_component_is_accepted():
    class Sub(MyComponent):
        pass

    with warnings.catch_warnings(record=True) as w:
        warnings.simplefilter("always")
        result = _validate_explicit_components([Sub])
    assert result == [Sub]
    assert w == []
