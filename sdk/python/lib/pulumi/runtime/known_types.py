# Copyright 2016-2018, Pulumi Corporation.
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
"""
The known_types module contains state for keeping track of types that
are known to be special in the Pulumi type system.

Python strictly disallows circular references between imported packages.
Because the Pulumi top-level module depends on the `pulumi.runtime` submodule,
it is not allowed for `pulumi.runtime` to reach back to the `pulumi` top-level
to reference types that are defined there.

In order to break this circular reference, and to be clear about what types
the runtime knows about and treats specially, this module exports a number of
"known type" decorators that can be applied to types in `pulumi` to indicate
that they are specially treated.

The implementation of this mechanism is that, for every known type, that type
is stashed away in a global variable. Whenever the runtime wants to do a type
test using that type (or instantiate an instance of this type), it uses the
functions defined in this module to do so.
"""
import six

_custom_resource_type = None
"""The type of CustomResource. Filled-in as the Pulumi package is initializing."""

_asset_resource_type = None
"""The type of Asset. Filled-in as the Pulumi package is initializing."""

_file_asset_resource_type = None
"""The type of FileAsset. Filled-in as the Pulumi package is initializing."""

_string_asset_resource_type = None
"""The type of StringAsset. Filled-in as the Pulumi package is initializing."""

_remote_asset_resource_type = None
"""The type of RemoteAsset. Filled-in as the Pulumi package is initializing."""

_custom_resource_type = None
"""The type of CustomResource. Filled-in as the Pulumi package is initializing."""

def asset(class_obj):
    """
    Decorator to annotate the Asset class. Registers the decorated class
    as the Asset known type.
    """
    assert isinstance(class_obj, six.class_types), "class_obj is not a Class"
    global _asset_resource_type
    _asset_resource_type = class_obj
    return class_obj

def file_asset(class_obj):
    """
    Decorator to annotate the FileAsset class. Registers the decorated class
    as the FileAsset known type.
    """
    assert isinstance(class_obj, six.class_types), "class_obj is not a Class"
    global _file_asset_resource_type
    _file_asset_resource_type = class_obj
    return class_obj

def string_asset(class_obj):
    """
    Decorator to annotate the StringAsset class. Registers the decorated class
    as the StringAsset known type.
    """
    assert isinstance(class_obj, six.class_types), "class_obj is not a Class"
    global _string_asset_resource_type
    _string_asset_resource_type = class_obj
    return class_obj

def remote_asset(class_obj):
    """
    Decorator to annotate the RemoteAsset class. Registers the decorated class
    as the RemoteAsset known type.
    """
    assert isinstance(class_obj, six.class_types), "class_obj is not a Class"
    global _remote_asset_resource_type 
    _remote_asset_resource_type = class_obj
    return class_obj

def custom_resource(class_obj):
    """
    Decorator to annotate the CustomResource class. Registers the decorated class
    as the CustomResource known type.
    """
    assert isinstance(class_obj, six.class_types), "class_obj is not a Class"
    global _custom_resource_type
    _custom_resource_type = class_obj
    return class_obj

def new_file_asset(*args):
    """
    Instantiates a new FileAsset, passing the given arguments to the constructor.
    """
    return _file_asset_resource_type(*args)

def new_string_asset(*args):
    """
    Instantiates a new StringAsset, passing the given arguments to the constructor.
    """
    return _string_asset_resource_type(*args)

def new_remote_asset(*args):
    """
    Instantiates a new StringAsset, passing the given arguments to the constructor.
    """
    return _remote_asset_resource_type(*args)


def is_asset(obj):
    """
    Returns true if the given type is an Asset, false otherwise.
    """
    return _asset_resource_type is not None and isinstance(obj, _asset_resource_type)

def is_custom_resource(obj):
    """
    Returns true if the given type is a CustomResource, false otherwise.
    """
    return _custom_resource_type is not None and isinstance(obj, _custom_resource_type)
