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
from typing import Optional, List, Any, Mapping, Union, TYPE_CHECKING

from .runtime import known_types
from .runtime.resource import register_resource, register_resource_outputs, read_resource
from .runtime.settings import get_root_resource

if TYPE_CHECKING:
    from .output import Output, Inputs


class ResourceOptions:
    """
    ResourceOptions is a bag of optional settings that control a resource's behavior.
    """

    parent: Optional['Resource']
    """
    If provided, the currently-constructing resource should be the child of the provided parent resource.
    """

    depends_on: Optional[List['Resource']]
    """
    If provided, the currently-constructing resource depends on the provided list of resources.
    """

    protect: Optional[bool]
    """
    If provided and True, this resource is not allowed to be deleted.
    """

    delete_before_replace: Optional[bool]
    """
    If provided and True, this resource must be deleted before it is replaced.
    """

    provider: Optional['ProviderResource']
    """
    An optional provider to use for this resource's CRUD operations. If no provider is supplied, the default
    provider for the resource's package will be used. The default provider is pulled from the parent's
    provider bag (see also ResourceOptions.providers).
    """

    providers: Union[Mapping[str, 'ProviderResource'], List['ProviderResource']]
    """
    An optional set of providers to use for child resources. Keyed by package name (e.g. "aws")
    """

    ignore_changes: Optional[List[str]]
    """
    If provided, ignore changes to any of the specified properties.
    """

    version: Optional[str]
    """
    An optional version. If provided, the engine loads a provider with exactly the requested version to operate on this
    resource. This version overrides the version information inferred from the current package and should rarely be
    used.
    """

    additional_secret_outputs: [List[str]]
    """
    The names of outputs for this resource that should be treated as secrets. This augments the list that
    the resource provider and pulumi engine already determine based on inputs to your resource. It can be used
    to mark certain ouputs as a secrets on a per resource basis.
    """

    id: Optional[str]
    """
    An optional existing ID to load, rather than create.
    """

    # pylint: disable=redefined-builtin
    def __init__(self,
                 parent: Optional['Resource'] = None,
                 depends_on: Optional[List['Resource']] = None,
                 protect: Optional[bool] = None,
                 provider: Optional['ProviderResource'] = None,
                 providers: Optional[Mapping[str, 'ProviderResource']] = None,
                 delete_before_replace: Optional[bool] = None,
                 ignore_changes: Optional[List[str]] = None,
                 version: Optional[str] = None,
                 additional_secret_outputs: Optional[List[str]] = None,
                 id: Optional[str] = None) -> None:
        """
        :param Optional[Resource] parent: If provided, the currently-constructing resource should be the child of
               the provided parent resource.
        :param Optional[List[Resource]] depends_on: If provided, the currently-constructing resource depends on the
               provided list of resources.
        :param Optional[bool] protect: If provided and True, this resource is not allowed to be deleted.
        :param Optional[ProviderResource] provider: An optional provider to use for this resource's CRUD operations.
               If no provider is supplied, the default provider for the resource's package will be used. The default
               provider is pulled from the parent's provider bag.
        :param Optional[Mapping[str,ProviderResource]] providers: An optional set of providers to use for child resources. Keyed
               by package name (e.g. "aws")
        :param Optional[bool] delete_before_replace: If provided and True, this resource must be deleted before it is replaced.
        :param Optional[List[string]] ignore_changes: If provided, a list of property names to ignore for purposes of updates
               or replacements.
        :param Optional[List[string]] additional_secret_outputs: If provided, a list of output property names that should
               also be treated as secret.
        :param Optional[str] id: If provided, an existing resource ID to read, rather than create.
        """
        self.parent = parent
        self.depends_on = depends_on
        self.protect = protect
        self.provider = provider
        self.providers = providers
        self.delete_before_replace = delete_before_replace
        self.ignore_changes = ignore_changes
        self.version = version
        self.additional_secret_outputs = additional_secret_outputs
        self.id = id

        if depends_on is not None:
            for dep in depends_on:
                if not isinstance(dep, Resource):
                    raise Exception("'dependsOn' was passed a value that was not a Resource.")

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
        """
        :param str t: The type of this resource.
        :param str name: The name of this resource.
        :param bool custom: True if this resource is a custom resource.
        :param Optional[dict] props: An optional list of input properties to use as inputs for the resource.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        """
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
                if not opts.parent is None:
                    # If no provider was given, but we have a parent, then inherit the
                    # provider from our parent.
                    opts.provider = opts.parent.get_provider(t)
            else:
                # If a provider was specified, add it to the providers map under this type's package so that
                # any children of this resource inherit its provider.
                type_components = t.split(":")
                if len(type_components) == 3:
                    [pkg, _, _] = type_components
                    self._providers = {**self._providers, pkg: provider}
        else:
            providers = self._convert_providers(opts.provider, opts.providers)
            self._providers = {**self._providers, **providers}

        self._protect = bool(opts.protect)

        if opts.id is not None:
            # If this resource already exists, read its state rather than registering it anow.
            if not custom:
                raise Exception("Cannot read an existing resource unless it has a custom provider")
            read_resource(self, t, name, props, opts)
        else:
            register_resource(self, t, name, custom, props, opts)

    def _convert_providers(self, provider: Optional['ProviderResource'], providers: Union[Mapping[str, 'ProviderResource'], List['ProviderResource']]) -> Mapping[str, 'ProviderResource']:
        if provider is not None:
            return self._convert_providers(None, [provider])

        if providers is None:
            return {}

        if not isinstance(providers, list):
            return providers

        result = {}
        for p in providers:
            result[p.package] = p

        return result

    def translate_output_property(self, prop: str) -> str:
        """
        Provides subclasses of Resource an opportunity to translate names of output properties
        into a format of their choosing before writing those properties to the resource object.

        :param str prop: A property name.
        :return: A potentially transformed property name.
        :rtype: str
        """
        return prop

    def translate_input_property(self, prop: str) -> str:
        """
        Provides subclasses of Resource an opportunity to translate names of input properties into
        a format of their choosing before sending those properties to the Pulumi engine.

        :param str prop: A property name.
        :return: A potentially transformed property name.
        :rtype: str
        """
        return prop

    def get_provider(self, module_member: str) -> Optional['ProviderResource']:
        """
        Fetches the provider for the given module member, if this resource has been provided a specific
        provider for the given module member.

        Returns None if no provider was provided.

        :param str module_member: The requested module member.
        :return: The :class:`ProviderResource` associated with the given module member, or None if one does not exist.
        :rtype: Optional[ProviderResource]
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

    __pulumi_type: str
    """
    Private field containing the type ID for this object. Useful for implementing `isInstance` on
    classes that inherit from `CustomResource`.
    """


    def __init__(self,
                 t: str,
                 name: str,
                 props: Optional[dict] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        """
        :param str t: The type of this resource.
        :param str name: The name of this resource.
        :param Optional[dict] props: An optional list of input properties to use as inputs for the resource.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        """
        Resource.__init__(self, t, name, True, props, opts)
        self.__pulumi_type = t


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
        """
        :param str t: The type of this resource.
        :param str name: The name of this resource.
        :param Optional[dict] props: An optional list of input properties to use as inputs for the resource.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        """
        Resource.__init__(self, t, name, False, props, opts)
        self.id = None

    def register_outputs(self, outputs):
        """
        Register synthetic outputs that a component has initialized, usually by allocating other child
        sub-resources and propagating their resulting property values.

        :param dict output: A dictionary of outputs to associate with this resource.
        """
        if outputs:
            register_resource_outputs(self, outputs)


class ProviderResource(CustomResource):
    """
    ProviderResource is a resource that implements CRUD operations for other custom resources. These resources are
    managed similarly to other resources, including the usual diffing and update semantics.
    """

    package: str
    """
    package is the name of the package this is provider for.  Common examples are "aws" and "azure".
    """


    def __init__(self,
                 pkg: str,
                 name: str,
                 props: Optional[dict] = None,
                 opts: Optional[ResourceOptions] = None) -> None:
        """
        :param str pkg: The package type of this provider resource.
        :param str name: The name of this resource.
        :param Optional[dict] props: An optional list of input properties to use as inputs for the resource.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        """

        if opts is not None and opts.provider is not None:
            raise TypeError("Explicit providers may not be used with provider resources")
        # Provider resources are given a well-known type, prefixed with "pulumi:providers".
        CustomResource.__init__(self, f"pulumi:providers:{pkg}", name, props, opts)
        self.package = pkg


def export(name: str, value: Any):
    """
    Exports a named stack output.

    :param str name: The name to assign to this output.
    :param Any value: The value of this output.
    """
    stack = get_root_resource()
    if stack is not None:
        stack.output(name, value)
