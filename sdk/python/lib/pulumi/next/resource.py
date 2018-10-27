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
from typing import Optional, List, Any

from ..runtime import known_types
from ..runtime.resource import register_resource, register_resource_outputs
from ..runtime.settings import get_root_resource
from ..runtime.unknown import Unknown


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

    """
    The stable, logical URN used to distinctly address a resource, both before and after deployments.
    """
    urn: str

    """
    The provider-assigned unique ID for this managed resource.  It is set during deployments and may
    be missing during planning phases.
    """
    id: str

    def __init__(self,
                 t: str,
                 name: str,
                 custom: bool,
                 props: Optional[dict] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        if not t:
            raise TypeError('Missing resource type argument')
        if not isinstance(t, str):
            raise TypeError('Expected resource type to be a string')
        if not name:
            raise TypeError('Missing resource name argument (for URN creation)')
        if not isinstance(name, str):
            raise TypeError('Expected resource name to be a string')

        # Properties and options can be missing; simply, initialize to empty dictionaries.
        if props:
            if not isinstance(props, dict):
                raise TypeError('Expected resource properties to be a dictionary')
        elif not props:
            props = dict()
        if opts:
            if not isinstance(opts, ResourceOptions):
                raise TypeError('Expected resource options to be a ResourceOptions instance')
        if not opts:
            opts = ResourceOptions()

        # Default the parent if there is none.
        if opts.parent is None:
            opts.parent = get_root_resource()

        # Now register the resource.  If we are actually performing a deployment, this resource's properties
        # will be resolved to real values.  If we are only doing a dry-run preview, on the other hand, they will
        # resolve to special Preview sentinel values to indicate the value isn't yet available.
        result = register_resource(t, name, custom, props, opts)

        # Set the URN, ID, and output properties.
        self.urn = result.urn
        if result.id:
            self.id = result.id
        else:
            self.id = Unknown()

        if result.outputs:
            self.set_outputs(result.outputs)

    def set_outputs(self, outputs: dict):
        """
        Sets output properties after a registration has completed.
        """


@known_types.custom_resource
class CustomResource(Resource):
    """
    CustomResource is a resource whose create, read, update, and delete (CRUD) operations are managed
    by performing external operations on some physical entity.  The engine understands how to diff
    and perform partial updates of them, and these CRUD operations are implemented in a dynamically
    loaded plugin for the defining package.
    """

    """
    id is the provider-assigned unique ID for this managed resource.  It is set during
    deployments and may be missing (undefined) during planning phases.
    """
    id: str

    """
    CustomResource is a resource whose CRUD operations are managed by performing external operations on some
    physical entity.  Pulumi understands how to diff and perform partial updates ot them, and these CRUD operations
    are implemented in a dynamically loaded plugin for the defining package.
    """
    def __init__(self,
                 t: str,
                 name: str,
                 props: Optional[dict] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
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


def output(name: str, value: Any):
    """
    Exports a named stack output.
    """
    stack = get_root_resource()
    if stack is not None:
        stack.output(name, value)
