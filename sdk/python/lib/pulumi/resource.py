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

"""The Resource module, containing all resource-related definitions."""
from typing import Optional, List, Any, TYPE_CHECKING

from .runtime import known_types
from .runtime.resource import register_resource, register_resource_outputs
from .runtime.settings import get_root_resource

if TYPE_CHECKING:
    from .output import Output, Inputs


class ResourceOptions:
    """
    ResourceOptions is a bag of optional settings that control a resource's behavior.
    """

    parent: Optional['Resource']
    depends_on: Optional[List['Resource']]
    protect: Optional[bool]

    def __init__(self,
                 parent: Optional['Resource'] = None,
                 depends_on: Optional[List['Resource']] = None,
                 protect: Optional[bool] = None) -> None:
        self.parent = parent
        self.depends_on = depends_on
        self.protect = protect


class Resource:
    """
    Resource represents a class whose CRUD operations are implemented by a provider plugin.
    """

    urn: 'Output[str]'
    """
    The stable, logical URN used to distinctly address a resource, both before and after deployments.
    """

    def __init__(self,
                 t: str,
                 name: str,
                 custom: bool,
                 props: Optional['Inputs'] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        if props is None:
            props = {}
        if not t:
            raise TypeError('Missing resource type argument')
        if not isinstance(t, str):
            raise TypeError('Expected resource type to be a string')
        if not name:
            raise TypeError('Missing resource name argument (for URN creation)')
        if not isinstance(name, str):
            raise TypeError('Expected resource name to be a string')

        # TODO(sean) first class providers here (pulumi/pulumi#1713)
        register_resource(self, t, name, custom, props, opts)

    def translate_output_property(self, prop: str) -> str:
        """
        Provides subclasses of Resource an opportunity to translate names of output properties
        into a format of their choosing before writing those properties to the resource object.
        """
        return prop

    def translate_input_property(self, prop: str) -> str:
        """
        Provides subclasses of Resource an opportunity to translate names of input properties into
        a format of their choosing before sending those properties to the Pulumi engine.
        """
        return prop



@known_types.custom_resource
class CustomResource(Resource):
    """
    CustomResource is a resource whose create, read, update, and delete (CRUD) operations are managed
    by performing external operations on some physical entity.  The engine understands how to diff
    and perform partial updates of them, and these CRUD operations are implemented in a dynamically
    loaded plugin for the defining package.
    """

    id: 'Output[str]'
    """
    id is the provider-assigned unique ID for this managed resource.  It is set during
    deployments and may be missing (undefined) during planning phases.
    """


    def __init__(self,
                 t: str,
                 name: str,
                 props: Optional[dict] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        """
        CustomResource is a resource whose CRUD operations are managed by performing external operations on some
        physical entity.  Pulumi understands how to diff and perform partial updates ot them, and these CRUD operations
        are implemented in a dynamically loaded plugin for the defining package.
        """
        Resource.__init__(self, t, name, True, props, opts)


class ComponentResource(Resource):
    """
    ComponentResource is a resource that aggregates one or more other child resources into a higher level
    abstraction.  The component itself is a resource, but does not require custom CRUD operations for provisioning.
    """
    def __init__(self,
                 t: str,
                 name: str,
                 props: Optional[dict] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        Resource.__init__(self, t, name, False, props, opts)
        self.id = None

    def register_outputs(self, outputs):
        """
        Register synthetic outputs that a component has initialized, usually by allocating other child
        sub-resources and propagating their resulting property values.
        """
        if outputs:
            register_resource_outputs(self, outputs)


def export(name: str, value: Any):
    """
    Exports a named stack output.
    """
    stack = get_root_resource()
    if stack is not None:
        stack.output(name, value)
