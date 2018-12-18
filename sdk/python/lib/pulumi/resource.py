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
from typing import Optional, List, Any, Mapping, TYPE_CHECKING

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

    provider: Optional['ProviderResource']
    """
    An optional provider to use for this resource's CRUD operations. If no provider is supplied, the default
    provider for the resource's package will be used. The default provider is pulled from the parent's
    provider bag (see also ResourceOptions.providers).
    """

    providers: Mapping[str, 'ProviderResource']
    """
    An optional set of providers to use for child resources. Keyed by package name (e.g. "aws")
    """


    def __init__(self,
                 parent: Optional['Resource'] = None,
                 depends_on: Optional[List['Resource']] = None,
                 protect: Optional[bool] = None,
                 provider: Optional['ProviderResource'] = None,
                 providers: Optional[Mapping[str, 'ProviderResource']] = None) -> None:
        self.parent = parent
        self.depends_on = depends_on
        self.protect = protect
        self.provider = provider
        self.providers = providers


class Resource:
    """
    Resource represents a class whose CRUD operations are implemented by a provider plugin.
    """

    urn: 'Output[str]'
    """
    The stable, logical URN used to distinctly address a resource, both before and after deployments.
    """

    _providers: Mapping[str, 'ProviderResource']
    """
    The set of providers to use for child resources. Keyed by package name (e.g. "aws").
    """

    _protect: bool
    """
    When set to true, protect ensures this resource cannot be deleted.
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
        if opts is None:
            opts = ResourceOptions()

        self._providers = {}
        # Check the parent type if one exists and fill in any default options.
        if opts.parent is not None:
            if not isinstance(opts.parent, Resource):
                raise TypeError("Resource parent is not a valid Resource")

            # Infer protection from parent, if one was provided.
            if opts.protect is None:
                opts.protect = opts.parent._protect

            # Infer providers and provider maps from parent, if one was provided.
            self._providers = opts.parent._providers
            if custom:
                provider = opts.provider
                if provider is None:
                    opts.provider = opts.parent.get_provider(t)
                else:
                    # If a provider was specified, add it to the providers map under this type's package so that
                    # any children of this resource inherit its provider.
                    type_components = t.split(":")
                    if len(type_components) == 3:
                        [pkg, _, _] = type_components
                        self._providers = {**self._providers, pkg: provider}

        if not custom:
            providers = opts.providers
            if providers is not None:
                self._providers = {**self._providers, **providers}

        self._protect = bool(opts.protect)
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

    def get_provider(self, module_member: str) -> Optional['ProviderResource']:
        """
        Fetches the provider for the given module member, if this resource has been provided a specific
        provider for the given module member.

        Returns None if no provider was provided.
        """
        components = module_member.split(":")
        if len(components) != 3:
            return None

        [pkg, _, _] = components
        return self._providers.get(pkg)



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


class ProviderResource(CustomResource):
    """
    ProviderResource is a resource that implements CRUD operations for other custom resources. These resources are
    managed similarly to other resources, including the usual diffing and update semantics.
    """
    def __init__(self,
                 pkg: str,
                 name: str,
                 props: Optional[dict] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        if opts is not None and opts.provider is not None:
            raise TypeError("Explicit providers may not be used with provider resources")
        # Provider resources are given a well-known type, prefixed with "pulumi:providers".
        CustomResource.__init__(self, f"pulumi:providers:{pkg}", name, props, opts)


def export(name: str, value: Any):
    """
    Exports a named stack output.
    """
    stack = get_root_resource()
    if stack is not None:
        stack.output(name, value)
