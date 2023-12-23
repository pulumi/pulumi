# Copyright 2016-2021, Pulumi Corporation.
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

import asyncio
import copy
import warnings
from typing import (
    Optional,
    List,
    Any,
    Mapping,
    Sequence,
    Union,
    Set,
    Callable,
    Tuple,
    TYPE_CHECKING,
    cast,
)
from . import _types
from .metadata import get_project, get_stack
from .runtime import known_types
from .runtime.resource import (
    _pkg_from_type,
    get_resource,
    register_resource,
    register_resource_outputs,
    read_resource,
    collapse_alias_to_urn,
    create_urn as create_urn_internal,
    convert_providers,
)
from .runtime.settings import get_root_resource
from .output import _is_prompt, _map_input, _map2_input, T, Output
from . import urn as urn_util
from . import log

if TYPE_CHECKING:
    from .output import Input, Inputs
    from .runtime.stack import Stack


class CustomTimeouts:
    create: Optional[str]
    """
    create is the optional create timout represented as a string e.g. 5m, 40s, 1d.
    """

    update: Optional[str]
    """
    update is the optional update timout represented as a string e.g. 5m, 40s, 1d.
    """

    delete: Optional[str]
    """
    delete is the optional delete timout represented as a string e.g. 5m, 40s, 1d.
    """

    def __init__(
        self,
        create: Optional[str] = None,
        update: Optional[str] = None,
        delete: Optional[str] = None,
    ) -> None:
        self.create = create
        self.update = update
        self.delete = delete


ROOT_STACK_RESOURCE = None
"""
Constant to represent the 'root stack' resource for a Pulumi application.  The purpose of this is
solely to make it easy to write an [Alias] like so:

`aliases=[Alias(parent=pulumi.ROOT_STACK_RESOURCE)]`.

This indicates that the prior name for a resource was created based on it being parented directly by
the stack itself and no other resources.  Note: this is equivalent to:

`aliases=[Alias(parent=None)]`

However, the former form is preferable as it is more self-descriptive, while the latter may look a
bit confusing and may incorrectly look like something that could be removed without changing
semantics.
"""


class Alias:
    """
    Alias is a partial description of prior named used for a resource. It can be processed in the
    context of a resource creation to determine what the full aliased URN would be.

    Note there is a semantic difference between attributes being given the `None` value and
    attributes not being given at all. Specifically, there is a difference between:

    ```ts
    Alias(name="foo", parent=None) # and
    Alias(name="foo")
    ```

    So the first alias means "the original urn had no parent" while the second alias means "use the
    current parent".

    Note: to indicate that a resource was previously parented by the root stack, it is recommended
    that you use:

    `aliases=[Alias(parent=pulumi.ROOT_STACK_RESOURCE)]`

    This form is self-descriptive and makes the intent clearer than using:

    `aliases=[Alias(parent=None)]`
    """

    name: Optional[str]
    """
    The previous name of the resource.  If not provided, the current name of the resource is used.
    """

    type_: Optional[str]
    """
    The previous type of the resource.  If not provided, the current type of the resource is used.
    """

    parent: Optional[Union["Resource", "Input[str]"]]
    """
    The previous parent of the resource.  If not provided (i.e. `Alias(name="foo")`), the current
    parent of the resource is used (`opts.parent` if provided, else the implicit stack resource
    parent).

    To specify no original parent, use `Alias(parent=pulumi.ROOT_STACK_RESOURCE)`.
    """

    stack: Optional["Input[str]"]
    """
    The name of the previous stack of the resource.  If not provided, defaults to `pulumi.getStack()`.
    """

    project: Optional["Input[str]"]
    """
    The previous project of the resource. If not provided, defaults to `pulumi.getProject()`.
    """

    # Ignoring type errors associated with the ellipsis constant being assigned to a string value.
    # We use it as a internal sentinel value, and don't need to expose this in the user facing type system.
    # https://docs.python.org/3/library/constants.html#Ellipsis
    def __init__(
        self,
        name: Optional[str] = ...,  # type: ignore
        type_: Optional[str] = ...,  # type: ignore
        parent: Optional[Union["Resource", "Input[str]"]] = ...,  # type: ignore
        stack: Optional["Input[str]"] = ...,  # type: ignore
        project: Optional["Input[str]"] = ...,  # type: ignore
    ) -> None:
        self.name = name
        self.type_ = type_
        self.parent = parent
        self.stack = stack
        self.project = project


class ResourceTransformationArgs:
    """
    ResourceTransformationArgs is the argument bag passed to a resource transformation.
    """

    resource: "Resource"
    """
    The Resource instance that is being transformed.
    """

    type_: str
    """
    The type of the Resource.
    """

    name: str
    """
    The name of the Resource.
    """

    props: "Inputs"
    """
    The original properties passed to the Resource constructor.
    """

    opts: "ResourceOptions"
    """
    The original resource options passed to the Resource constructor.
    """

    def __init__(
        self,
        resource: "Resource",
        type_: str,
        name: str,
        props: "Inputs",
        opts: "ResourceOptions",
    ) -> None:
        self.resource = resource
        self.type_ = type_
        self.name = name
        self.props = props
        self.opts = opts


class ResourceTransformationResult:
    """
    ResourceTransformationResult is the result that must be returned by a resource transformation
    callback.  It includes new values to use for the `props` and `opts` of the `Resource` in place of
    the originally provided values.
    """

    props: "Inputs"
    """
    The new properties to use in place of the original `props`.
    """

    opts: "ResourceOptions"
    """
    The new resource options to use in place of the original `opts`
    """

    def __init__(self, props: "Inputs", opts: "ResourceOptions") -> None:
        self.props = props
        self.opts = opts


ResourceTransformation = Callable[
    [ResourceTransformationArgs], Optional[ResourceTransformationResult]
]
"""
ResourceTransformation is the callback signature for the `transformations` resource option.  A
transformation is passed the same set of inputs provided to the `Resource` constructor, and can
optionally return back alternate values for the `props` and/or `opts` prior to the resource
actually being created.  The effect will be as though those props and opts were passed in place
of the original call to the `Resource` constructor.  If the transformation returns None,
this indicates that the resource will not be transformed.
"""


class ResourceTransformArgs:
    """
    ResourceTransformArgs is the argument bag passed to a resource transform.
    """

    custom: bool
    """
    If the resource is a custom or component resource.
    """

    type_: str
    """
    The type of the Resource.
    """

    name: str
    """
    The name of the Resource.
    """

    props: "Inputs"
    """
    The original properties passed to the Resource constructor.
    """

    opts: "ResourceOptions"
    """
    The original resource options passed to the Resource constructor.
    """

    def __init__(
        self,
        custom: bool,
        type_: str,
        name: str,
        props: "Inputs",
        opts: "ResourceOptions",
    ) -> None:
        self.custom = custom
        self.type_ = type_
        self.name = name
        self.props = props
        self.opts = opts


class ResourceTransformResult:
    """
    ResourceTransformResult is the result that must be returned by a resource transform callback.
    It includes new values to use for the `props` and `opts` of the `Resource` in place of the
    originally provided values.
    """

    props: "Inputs"
    """
    The new properties to use in place of the original `props`.
    """

    opts: "ResourceOptions"
    """
    The new resource options to use in place of the original `opts`
    """

    def __init__(self, props: "Inputs", opts: "ResourceOptions") -> None:
        self.props = props
        self.opts = opts


ResourceTransform = Callable[[ResourceTransformArgs], Optional[ResourceTransformResult]]
"""
ResourceTransform is the callback signature for the `transforms` resource option.  A
transform is passed the same set of inputs provided to the `Resource` constructor, and can
optionally return back alternate values for the `props` and/or `opts` prior to the resource
actually being created.  The effect will be as though those props and opts were passed in place
of the original call to the `Resource` constructor.  If the transform returns None,
this indicates that the resource will not be transformed.
"""


class ResourceOptions:
    """
    ResourceOptions is a bag of optional settings that control a resource's behavior.
    """

    parent: Optional["Resource"]
    """
    If provided, the currently-constructing resource should be the child of the provided parent
    resource.
    """

    depends_on: Optional["Input[Union[Sequence[Input[Resource]], Resource]]"]
    """
    If provided, declares that the currently-constructing resource depends on the given resources.
    """

    protect: Optional[bool]
    """
    If provided and True, this resource is not allowed to be deleted.
    """

    delete_before_replace: Optional[bool]
    """
    If provided and True, this resource must be deleted before it is replaced.
    """

    provider: Optional["ProviderResource"]
    """
    An optional provider to use for this resource's CRUD operations. If no provider is supplied, the
    default provider for the resource's package will be used. The default provider is pulled from
    the parent's provider bag (see also ResourceOptions.providers).
    """

    providers: Optional[
        Union[Mapping[str, "ProviderResource"], Sequence["ProviderResource"]]
    ]
    """
    An optional set of providers to use for this resource and child resources. Keyed by package name
    (e.g. "aws"), or just provided as a list. In the latter case, the package name will be retrieved
    from the provider itself.
    Note: Only a list should be used. Mapping keys are not respected.
    """

    ignore_changes: Optional[List[str]]
    """
    If provided, ignore changes to any of the specified properties.
    """

    version: Optional[str]
    """
    An optional version. If provided, the engine loads a provider with exactly the requested version
    to operate on this resource. This version overrides the version information inferred from the
    current package and should rarely be used.
    """

    plugin_download_url: Optional[str]
    """
    An optional url. If provided, the engine loads a provider with downloaded from the provided url.
    This url overrides the plugin download url inferred from the current package and should rarely
    be used.
    """

    aliases: Optional[Sequence["Input[Union[str, Alias]]"]]
    """
    An optional list of aliases to treat this resource as matching.
    """

    additional_secret_outputs: Optional[List[str]]
    """
    The names of outputs for this resource that should be treated as secrets. This augments the list
    that the resource provider and pulumi engine already determine based on inputs to your resource.
    It can be used to mark certain outputs as a secrets on a per resource basis.
    """

    custom_timeouts: Optional["CustomTimeouts"]
    """
    An optional customTimeouts config block.
    """

    transformations: Optional[List[ResourceTransformation]]
    """
    Optional list of transformations to apply to this resource during construction. The
    transformations are applied in order, and are applied prior to transformation applied to
    parents walking from the resource up to the stack.
    """

    x_transforms: Optional[List[ResourceTransform]]
    """
    Optional list of transforms to apply to this resource during construction. The
    transforms are applied in order, and are applied prior to transform applied to
    parents walking from the resource up to the stack.

    This is experimental.
    """

    id: Optional["Input[str]"]
    """
    An optional existing ID to load, rather than create.
    """

    import_: Optional[str]
    """
    When provided with a resource ID, import indicates that this resource's provider should import
    its state from the cloud resource with the given ID. The inputs to the resource's constructor
    must align with the resource's current state. Once a resource has been imported, the import
    property must be removed from the resource's options.
    """

    urn: Optional[str]
    """
    The URN of a previously-registered resource of this type to read from the engine.
    """

    replace_on_changes: Optional[List[str]]
    """
    Changes to any of these property paths will force a replacement.  If this list includes `"*"`, changes
    to any properties will force a replacement.  Initialization errors from previous deployments will
    require replacement instead of update only if `"*"` is passed.
    """

    retain_on_delete: Optional[bool]
    """
    If set to True, the providers Delete method will not be called for this resource.
    """

    deleted_with: Optional["Resource"]
    """
    If set, the providers Delete method will not be called for this resource
    if specified resource is being deleted as well.
    """

    # pylint: disable=redefined-builtin
    def __init__(
        self,
        parent: Optional["Resource"] = None,
        depends_on: Optional[
            "Input[Union[Sequence[Input[Resource]], Resource]]"
        ] = None,
        protect: Optional[bool] = None,
        provider: Optional["ProviderResource"] = None,
        providers: Optional[
            Union[Mapping[str, "ProviderResource"], List["ProviderResource"]]
        ] = None,
        delete_before_replace: Optional[bool] = None,
        ignore_changes: Optional[List[str]] = None,
        version: Optional[str] = None,
        aliases: Optional[Sequence["Input[Union[str, Alias]]"]] = None,
        additional_secret_outputs: Optional[List[str]] = None,
        id: Optional["Input[str]"] = None,
        import_: Optional[str] = None,
        custom_timeouts: Optional["CustomTimeouts"] = None,
        transformations: Optional[List[ResourceTransformation]] = None,
        x_transforms: Optional[List[ResourceTransform]] = None,
        urn: Optional[str] = None,
        replace_on_changes: Optional[List[str]] = None,
        plugin_download_url: Optional[str] = None,
        retain_on_delete: Optional[bool] = None,
        deleted_with: Optional["Resource"] = None,
    ) -> None:
        """
        :param Optional[Resource] parent: If provided, the currently-constructing resource should be the child of
               the provided parent resource.
        :param Optional[Input[Union[List[Input[Resource]],Resource]]] depends_on: If provided, declares that the
               currently-constructing resource depends on the given resources.
        :param Optional[bool] protect: If provided and True, this resource is not allowed to be deleted.
        :param Optional[ProviderResource] provider: An optional provider to use for this resource's CRUD operations.
               If no provider is supplied, the default provider for the resource's package will be used. The default
               provider is pulled from the providers field, then the parent's provider bag.
        :param Optional[Union[Mapping[str, ProviderResource], List[ProviderResource]]] providers: An optional set of
               providers to use for this resource and child resources. Keyed by package name (e.g. "aws"), or just
               provided as a list. In the latter case, the package name will be retrieved from the provider itself.
               Note: Only a list should be used. Mapping keys are not respected.
        :param Optional[bool] delete_before_replace: If provided and True, this resource must be deleted before it is replaced.
        :param Optional[List[str]] ignore_changes: If provided, a list of property names to ignore for purposes of updates
               or replacements.
        :param Optional[str] version: An optional version. If provided, the engine loads a provider with exactly the
               requested version to operate on this resource. This version overrides the version information inferred
               from the current package and should rarely be used.
        :param Optional[List[Input[Union[str, Alias]]]] aliases: An optional list of aliases to treat this resource as
               matching.
        :param Optional[List[str]] additional_secret_outputs: If provided, a list of output property names that should
               also be treated as secret.
        :param Optional[Input[str]] id: If provided, an existing resource ID to read, rather than create.
        :param Optional[str] import_: When provided with a resource ID, import indicates that this resource's provider should
               import its state from the cloud resource with the given ID. The inputs to the resource's constructor must align
               with the resource's current state. Once a resource has been imported, the import property must be removed from
               the resource's options.
        :param Optional[CustomTimeouts] custom_timeouts: If provided, a config block for custom timeout information.
        :param Optional[List[ResourceTransformation]] transformations: If provided, a list of transformations to apply
               to this resource during construction.
        :param Optional[List[ResourceTransform]] x_transforms: If provided, a list of transforms to apply
               to this resource during construction. This is experimental.
        :param Optional[str] urn: The URN of a previously-registered resource of this type to read from the engine.
        :param Optional[List[str]] replace_on_changes: Changes to any of these property paths will force a replacement.
               If this list includes `"*"`, changes to any properties will force a replacement.  Initialization errors
               from previous deployments will require replacement instead of update only if `"*"` is passed.
        :param Optional[str] plugin_download_url: An optional url. If provided, the engine loads a provider with downloaded
               from the provided url. This url overrides the plugin download url inferred from the current package and should
               rarely be used.
        :param Optional[bool] retain_on_delete: If set to True, the providers Delete method will not be called for this resource.
        :param Optional[Resource] deleted_with: If set, the providers Delete method will not be called for this resource
               if specified resource is being deleted as well.
        """

        # Expose 'merge' again this this object, but this time as an instance method.
        # TODO[python/mypy#2427]: mypy disallows method assignment
        self.merge = self._merge_instance  # type: ignore
        self.merge.__func__.__doc__ = ResourceOptions.merge.__doc__  # type: ignore

        self.parent = parent
        self.protect = protect
        self.provider = provider
        self.providers = providers
        self.delete_before_replace = delete_before_replace
        self.ignore_changes = ignore_changes
        self.version = version
        self.plugin_download_url = plugin_download_url
        self.aliases = aliases
        self.additional_secret_outputs = additional_secret_outputs
        self.custom_timeouts = custom_timeouts
        self.id = id
        self.import_ = import_
        self.transformations = transformations
        self.x_transforms = x_transforms
        self.urn = urn
        self.replace_on_changes = replace_on_changes
        self.depends_on = depends_on
        self.retain_on_delete = retain_on_delete
        self.deleted_with = deleted_with

        # Proactively check that `depends_on` values are of type
        # `Resource`. We cannot complete the check in the general case
        # and can only do it on promptly available arguments.
        deps = self._depends_on_list()
        if isinstance(deps, list):
            for dep in deps:
                if _is_prompt(dep) and not isinstance(dep, Resource):
                    raise Exception(
                        f"'depends_on' was passed a value {dep} that was not a Resource."
                    )

    def _merge_instance(self, opts: "ResourceOptions") -> "ResourceOptions":
        return ResourceOptions.merge(self, opts)

    def _depends_on_list(self) -> "Input[List[Input[Resource]]]":
        if self.depends_on is None:
            return []

        return _map_input(
            self.depends_on,
            lambda x: list(x) if isinstance(x, Sequence) else [cast(Any, x)],
        )

    def _copy(self) -> "ResourceOptions":
        """
        Copy a ResourceOptions.
        """

        # ResourceOptions.merge is a static method that is reassigned to a field, not a
        # true method. This assignment captures the original class. When we copy a
        # ResourceOptions, we need update merge to point to the new class.
        #
        # We can illustrate this with an example.
        #
        # instance_1 of type ResourceOptions :
        #   merge = instance_1._instance_merge
        #   id: id_1
        #   # other fields:
        #   ...
        #
        # instance_2 = copy.copy(instance_1):
        #   merge = instance_1._instance_merge
        #   id: id_1
        #   # other fields
        #   ...
        #
        # instance_2.parent = parent_2
        #
        # instance_3 = instance_2.merge(None)
        #
        # We get `instance_3.parent == None`, when we should get `instance_3.parent ==
        # parent_2`. This is because merge had a handle to instance_1, instead of
        # instance_2.
        #
        # The fix is to re-assign merge after the copy.

        out = copy.copy(self)
        # Expose 'merge' again this this object, but this time as an instance method.
        # TODO[python/mypy#2427]: mypy disallows method assignment
        out.merge = out._merge_instance  # type: ignore
        return out

    # pylint: disable=method-hidden
    @staticmethod
    def merge(
        opts1: Optional["ResourceOptions"], opts2: Optional["ResourceOptions"]
    ) -> "ResourceOptions":
        """
        merge produces a new ResourceOptions object with the respective attributes of the `opts1`
        instance in it with the attributes of `opts2` merged over them.

        Both the `opts1` instance and the `opts2` instance will be unchanged.  Both of `opts1` and
        `opts2` can be `None`, in which case its attributes are ignored.

        Conceptually attributes merging follows these basic rules:

        1. If the attributes is a collection, the final value will be a collection containing the
           values from each options object. Both original collections in each options object will
           be unchanged.

        2. Simple scalar values from `opts2` (i.e. strings, numbers, bools) will replace the values
           from `opts1`.

        3. For the purposes of merging `depends_on` is always treated
           as collections, even if only a single value was provided.

        4. Attributes with value 'None' will not be copied over.

        This method can be called either as static-method like `ResourceOptions.merge(opts1, opts2)`
        or as an instance-method like `opts1.merge(opts2)`.  The former is useful for cases where
        `opts1` may be `None` so the caller does not need to check for this case.
        """

        opts1 = ResourceOptions() if opts1 is None else opts1
        opts2 = ResourceOptions() if opts2 is None else opts2

        if not isinstance(opts1, ResourceOptions):
            raise TypeError("Expected opts1 to be a ResourceOptions instance")

        if not isinstance(opts2, ResourceOptions):
            raise TypeError("Expected opts2 to be a ResourceOptions instance")

        dest = opts1._copy()
        source = opts2._copy()

        # This makes merging simple.
        def expand_providers(providers) -> Optional[Sequence["ProviderResource"]]:
            if isinstance(providers, Mapping):
                for k, p in providers.items():
                    if k != p.package:
                        message = f"Provider map key {k} disagrees with associated provider {p.package}. Key will be ignored."
                        warnings.warn(message, UserWarning)
                        log.warn(message)
                return list(providers.values())
            return providers

        dest_providers = expand_providers(dest.providers)
        source_providers = expand_providers(source.providers)

        dest.providers = _merge_lists(dest_providers, source_providers)

        dest.depends_on = _map2_input(
            dest._depends_on_list(), source._depends_on_list(), lambda xs, ys: xs + ys
        )

        dest.ignore_changes = _merge_lists(dest.ignore_changes, source.ignore_changes)
        dest.replace_on_changes = _merge_lists(
            dest.replace_on_changes, source.replace_on_changes
        )
        dest.aliases = _merge_lists(dest.aliases, source.aliases)
        dest.additional_secret_outputs = _merge_lists(
            dest.additional_secret_outputs, source.additional_secret_outputs
        )
        dest.transformations = _merge_lists(
            dest.transformations, source.transformations
        )
        dest.x_transforms = _merge_lists(dest.x_transforms, source.x_transforms)

        dest.parent = dest.parent if source.parent is None else source.parent
        dest.protect = dest.protect if source.protect is None else source.protect
        dest.delete_before_replace = (
            dest.delete_before_replace
            if source.delete_before_replace is None
            else source.delete_before_replace
        )
        dest.version = dest.version if source.version is None else source.version
        dest.plugin_download_url = (
            dest.plugin_download_url
            if source.plugin_download_url is None
            else source.plugin_download_url
        )
        dest.custom_timeouts = (
            dest.custom_timeouts
            if source.custom_timeouts is None
            else source.custom_timeouts
        )
        dest.id = dest.id if source.id is None else source.id
        dest.import_ = dest.import_ if source.import_ is None else source.import_
        dest.urn = dest.urn if source.urn is None else source.urn
        dest.provider = dest.provider if source.provider is None else source.provider
        dest.retain_on_delete = (
            dest.retain_on_delete
            if source.retain_on_delete is None
            else source.retain_on_delete
        )
        dest.deleted_with = (
            dest.deleted_with if source.deleted_with is None else source.deleted_with
        )

        # Now, if we are left with a .providers that is just a single key/value pair, then
        # collapse that down into .provider form.
        _collapse_providers(dest)

        return dest


def _collapse_providers(opts: "ResourceOptions"):
    """
    If we have 0 providers, we set .providers to None. Otherwise, we ensure that
    .providers is returned as a map.
    """
    providers: Optional[
        Union[Mapping[str, ProviderResource], Sequence[ProviderResource]]
    ] = opts.providers
    if providers is not None:
        provider_length = len(providers)
        if provider_length == 0:
            opts.providers = None
        else:
            opts.providers = {}
            if isinstance(providers, list):
                for prov in providers:
                    opts.providers[prov.package] = prov
            elif isinstance(providers, dict):
                for key, prov in providers:
                    opts.providers[key] = prov


def _merge_lists(dest, source):
    if dest is None:
        return source

    if source is None:
        return dest

    return dest + source


# !!! IMPORTANT !!! If you add a new attribute to this type, make sure to verify that ResourceOptions.merge
# works properly for it.
class Resource:
    """
    Resource represents a class whose CRUD operations are implemented by a provider plugin.
    """

    _providers: Mapping[str, "ProviderResource"]
    """
    The set of providers to use for child resources. Keyed by package name (e.g. "aws").
    """

    _provider: Optional["ProviderResource"]
    """
    The specified provider or provider determined from the parent for custom resources, or None.
    """

    _version: Optional[str]
    """
    The specified provider version or None.
    """

    _protect: bool
    """
    When set to true, protect ensures this resource cannot be deleted.
    """

    _transformations: "List[ResourceTransformation]"
    """
    A collection of transformations to apply as part of resource registration.
    """

    _aliases: "List[Input[str]]"
    """
    A list of aliases applied to this resource.
    """

    _name: str
    """
    The name assigned to the resource at construction.
    """

    _type: str
    """
    The type of the resource.
    """

    _plugin_download_url: Optional[str]
    """
    The specified download URL associated with the provider or None.
    """

    _childResources: Set["Resource"]

    # !!! IMPORTANT !!! If you add a new attribute to this type, make sure to verify that ResourceOptions.merge
    # works properly for it.

    def __init__(
        self,
        t: str,
        name: str,
        custom: bool,
        props: Optional["Inputs"] = None,
        opts: Optional[ResourceOptions] = None,
        remote: bool = False,
        dependency: bool = False,
    ) -> None:
        """
        :param str t: The type of this resource.
        :param str name: The name of this resource.
        :param bool custom: True if this resource is a custom resource.
        :param Optional[Inputs] props: An optional list of input properties to use as inputs for the resource.
               If props is an input type (decorated with `@input_type`), dict keys will be translated using
               the type's and resource's type/name metadata rather than using the `translate_input_property`
               and `translate_output_property` methods.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        :param bool remote: True if this is a remote component resource.
        :param bool dependency: True if this is a synthetic resource used internally for dependency tracking.
        """

        if dependency:
            self._protect = False
            self._providers = {}
            return

        if props is None:
            props = {}
        if not t:
            raise TypeError("Missing resource type argument")
        if not isinstance(t, str):
            raise TypeError("Expected resource type to be a string")
        if not name:
            raise TypeError("Missing resource name argument (for URN creation)")
        if not isinstance(name, str):
            raise TypeError("Expected resource name to be a string")
        if opts is None:
            opts = ResourceOptions()
        elif not isinstance(opts, ResourceOptions):
            raise TypeError(
                "Expected resource options to be a ResourceOptions instance"
            )

        # If `props` is an input type, convert it into an untranslated dictionary.
        # Translation of the keys will happen later using the type's and resource's type/name metadata.
        # If `props` is not an input type, set `typ` to None to make translation behave as it has previously.
        typ: Optional[type] = type(props)
        assert typ is not None
        if _types.is_input_type(typ):
            props = _types.input_type_to_untranslated_dict(props)
        else:
            if not isinstance(props, Mapping):
                raise TypeError("Expected resource properties to be a mapping")
            typ = None

        # Before anything else - if there are transformations registered, give them a chance to run to modify the user
        # provided properties and options assigned to this resource.
        parent = opts.parent
        if parent is None:
            parent = get_root_resource()
        parent_transformations = (
            (parent._transformations or []) if parent is not None else []
        )
        self._transformations = (opts.transformations or []) + parent_transformations
        for transformation in self._transformations:
            args = ResourceTransformationArgs(
                resource=self, type_=t, name=name, props=props, opts=opts
            )
            tres = transformation(args)
            if tres is not None:
                if tres.opts.parent != opts.parent:
                    # This is currently not allowed because the parent tree is needed to establish what
                    # transformation to apply in the first place, and to compute inheritance of other
                    # resource options in the Resource constructor before transformations are run (so
                    # modifying it here would only even partially take affect).  It's theoretically
                    # possible this restriction could be lifted in the future, but for now just
                    # disallow re-parenting resources in transformations to be safe.
                    raise Exception(
                        "Transformations cannot currently be used to change the `parent` of a resource."
                    )
                props = tres.props
                opts = tres.opts

        self._name = name
        self._type = t

        # Make a shallow clone of opts to ensure we don't modify the value passed in.
        opts = opts._copy()

        self._aliases = []
        if opts is not None and opts.aliases is not None:
            for alias in opts.aliases:
                self._aliases.append(collapse_alias_to_urn(alias, name, t, opts.parent))

        self._providers = {}
        self._childResources = set()

        # Check the parent type if one exists and fill in any default options.
        if opts.parent is not None:
            if not isinstance(opts.parent, Resource):
                raise TypeError("Resource parent is not a valid Resource")

            # Add this resource to its parent's set of child resources.
            opts.parent._childResources.add(self)

            # Infer protection from parent, if one was provided.
            if opts.protect is None:
                opts.protect = opts.parent._protect

            # Infer providers and provider maps from parent, if one was provided.
            self._providers = opts.parent._providers

        pkg = _pkg_from_type(t)
        opts.provider, opts.providers = self._get_providers(t, pkg, opts)

        self._protect = bool(opts.protect)
        self._provider = opts.provider if (custom or remote) else None
        if self._provider and self._provider.package != pkg:
            action = (
                "get"
                if opts.urn is not None
                else "read" if opts.id is not None else "register"
            )
            raise ValueError(
                f"Attempted to {action} resource {t} with a provider for '{self._provider.package}'"
            )
        self._providers = opts.providers
        self._version = opts.version
        self._plugin_download_url = opts.plugin_download_url
        if opts.urn is not None:
            # This is a resource that already exists. Read its state from the engine.
            get_resource(self, props, custom, opts.urn, typ)
        elif opts.id is not None:
            # If this is a custom resource that already exists, read its state from the provider.
            if not custom:
                raise Exception(
                    "Cannot read an existing resource unless it has a custom provider"
                )
            read_resource(cast("CustomResource", self), t, name, props, opts, typ)
        else:
            register_resource(
                self, t, name, custom, remote, DependencyResource, props, opts, typ
            )

    def _get_providers(
        self, t: str, pkg: Optional[str], opts: ResourceOptions
    ) -> Tuple[Optional["ProviderResource"], Mapping[str, "ProviderResource"]]:
        """
        Fetches the correct provider and providers for this resource.

        provider is the first option that does not return none
            1. opts.provider
            2. a matching provider in opts.providers
            3. a matching provider inherited from opts.parent

        providers is found by combining (in descending order of priority)
            1. opts_providers
            2. self_providers
            3. provider

        Does not mutate self.
        """
        if isinstance(opts.providers, Sequence):
            opts_providers = {p.package: p for p in opts.providers}
        elif opts.providers:
            opts_providers = {**opts.providers}
        else:
            opts_providers = {}

        # The provider provided by opts.providers
        ambient_provider: Optional[ProviderResource] = None
        if pkg and pkg in opts_providers:
            ambient_provider = opts_providers[pkg]

        # The provider supplied by the parent

        # Cast is safe because the `and` will resolve to only None or the result
        # of get_provider (which is Optional[ProviderResource]). This holds as
        # long as Resource does not impliment __bool__.
        parent_provider = cast(
            Optional[ProviderResource], opts.parent and opts.parent.get_provider(t)
        )

        provider = ambient_provider or parent_provider
        if opts.provider:
            # If an explicit provider was passed in,
            # its package may or may not match the package we're looking for.
            provider_pkg = opts.provider.package
            if pkg == provider_pkg:
                # Explicit provider takes precedence over parent or ambient providers.
                provider = opts.provider

            # Regardless of whether it matches,
            # we still want to keep the provider in the
            # providers map, so that it can be used for child resources.
            if provider_pkg in opts_providers:
                message = f"There is a conflict between the `provider` field ({provider_pkg}) and a member of the `providers` map"
                depreciation = "This will become an error in a future version. See https://github.com/pulumi/pulumi/issues/8799 for more details"
                warnings.warn(f"{message} for resource {t}. " + depreciation)
                log.warn(f"{message}. {depreciation}", resource=self)
            else:
                opts_providers[provider_pkg] = opts.provider

        # opts_providers takes priority over self._providers
        providers = {**self._providers, **opts_providers}

        return provider, providers

    @property
    def urn(self) -> "Output[str]":
        """
        The stable, logical URN used to distinctly address a resource, both before and after
        deployments.
        """
        return self.__dict__["urn"]

    def translate_output_property(self, prop: str) -> str:
        """
        Provides subclasses of Resource an opportunity to translate names of output properties
        into a format of their choosing before writing those properties to the resource object.

        If the `props` passed to `__init__` is an input type (decorated with `@input_type`), the
        type/name metadata of the resource will be used to translate names instead of calling this
        method.

        :param str prop: A property name.
        :return: A potentially transformed property name.
        :rtype: str
        """
        return prop

    def translate_input_property(self, prop: str) -> str:
        """
        Provides subclasses of Resource an opportunity to translate names of input properties into
        a format of their choosing before sending those properties to the Pulumi engine.

        If the `props` passed to `__init__` is an input type (decorated with `@input_type`), the
        type/name metadata of `props` will be used to translate names instead of calling this
        method.

        :param str prop: A property name.
        :return: A potentially transformed property name.
        :rtype: str
        """
        return prop

    def get_provider(self, module_member: str) -> Optional["ProviderResource"]:
        """
        Fetches the provider for the given module member, if this resource has been provided a specific
        provider for the given module member.

        Returns None if no provider was provided.

        :param str module_member: The requested module member.
        :return: The :class:`ProviderResource` associated with the given module member, or None if one does not exist.
        :rtype: Optional[ProviderResource]
        """
        pkg = _pkg_from_type(module_member)
        if pkg is None:
            return None

        return self._providers.get(pkg)


class CustomResource(Resource):
    """
    CustomResource is a resource whose create, read, update, and delete (CRUD) operations are
    managed by performing external operations on some physical entity.  The engine understands how
    to diff and perform partial updates of them, and these CRUD operations are implemented in a
    dynamically loaded plugin for the defining package.
    """

    def __init__(
        self,
        t: str,
        name: str,
        props: Optional["Inputs"] = None,
        opts: Optional[ResourceOptions] = None,
        dependency: bool = False,
    ) -> None:
        """
        :param str t: The type of this resource.
        :param str name: The name of this resource.
        :param Optional[dict] props: An optional list of input properties to use as inputs for the resource.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        :param bool dependency: True if this is a synthetic resource used internally for dependency tracking.
        """
        Resource.__init__(self, t, name, True, props, opts, False, dependency)

    @property
    def id(self) -> "Output[str]":
        """
        id is the provider-assigned unique ID for this managed resource.  It is set during
        deployments and may be missing (undefined) during planning phases.
        """
        return self.__dict__["id"]


class ComponentResource(Resource):
    """
    ComponentResource is a resource that aggregates one or more other child resources into a higher
    level abstraction.  The component itself is a resource, but does not require custom CRUD
    operations for provisioning.
    """

    _remote: bool

    def __init__(
        self,
        t: str,
        name: str,
        props: Optional["Inputs"] = None,
        opts: Optional[ResourceOptions] = None,
        remote: bool = False,
    ) -> None:
        """
        :param str t: The type of this resource.
        :param str name: The name of this resource.
        :param Optional[dict] props: An optional list of input properties to use as inputs for the resource.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        :param bool remote: True if this is a remote component resource.
        """
        Resource.__init__(self, t, name, False, props, opts, remote, False)
        if not remote:
            self.__dict__["id"] = None
        self._remote = remote

    def register_outputs(self, outputs: "Inputs"):
        """
        Register synthetic outputs that a component has initialized, usually by allocating other child
        sub-resources and propagating their resulting property values.

        :param dict output: A dictionary of outputs to associate with this resource.
        """
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

    def __init__(
        self,
        pkg: str,
        name: str,
        props: Optional["Inputs"] = None,
        opts: Optional[ResourceOptions] = None,
        dependency: bool = False,
    ) -> None:
        """
        :param str pkg: The package type of this provider resource.
        :param str name: The name of this resource.
        :param Optional[dict] props: An optional list of input properties to use as inputs for the resource.
        :param Optional[ResourceOptions] opts: Optional set of :class:`pulumi.ResourceOptions` to use for this
               resource.
        :param bool dependency: True if this is a synthetic resource used internally for dependency tracking.
        """

        if opts is not None and opts.provider is not None:
            raise TypeError(
                "Explicit providers may not be used with provider resources"
            )
        # Provider resources are given a well-known type, prefixed with "pulumi:providers".
        CustomResource.__init__(
            self, f"pulumi:providers:{pkg}", name, props, opts, dependency
        )
        self.package = pkg


class DependencyResource(CustomResource):
    """
    A DependencyResource is a resource that is used to indicate that an Output has a dependency on a particular
    resource. These resources are only created when dealing with remote component resources.
    """

    def __init__(self, urn: str) -> None:
        super().__init__(t="", name="", props={}, opts=None, dependency=True)

        urn_future: asyncio.Future[str] = asyncio.Future()
        urn_known: asyncio.Future[bool] = asyncio.Future()
        urn_secret: asyncio.Future[bool] = asyncio.Future()
        urn_future.set_result(urn)
        urn_known.set_result(True)
        urn_secret.set_result(False)
        self.__dict__["urn"] = Output({self}, urn_future, urn_known, urn_secret)


class DependencyProviderResource(ProviderResource):
    """
    A DependencyProviderResource is a resource that is used by the provider SDK as a stand-in for a provider that
    is only used for its reference. Its only valid properties are its URN and ID.
    """

    def __init__(self, ref: str) -> None:
        ref_urn, ref_id = _parse_resource_reference(ref)
        urn_parts = urn_util._parse_urn(ref_urn)

        # `typ` will be `pulumi:providers:<package>` and we want the
        # last part, which normally parses as `typ_name`.
        pkg = urn_parts.typ_name

        super().__init__(pkg=pkg, name="", props={}, opts=None, dependency=True)

        urn_future: asyncio.Future[str] = asyncio.Future()
        urn_known: asyncio.Future[bool] = asyncio.Future()
        urn_secret: asyncio.Future[bool] = asyncio.Future()
        urn_future.set_result(ref_urn)
        urn_known.set_result(True)
        urn_secret.set_result(False)
        self.__dict__["urn"] = Output({self}, urn_future, urn_known, urn_secret)

        id_future: asyncio.Future[str] = asyncio.Future()
        id_known: asyncio.Future[bool] = asyncio.Future()
        id_secret: asyncio.Future[bool] = asyncio.Future()
        id_future.set_result(ref_id)
        id_known.set_result(True)
        id_secret.set_result(False)
        self.__dict__["id"] = Output({self}, id_future, id_known, id_secret)


def export(name: str, value: Any):
    """
    Exports a named stack output.

    :param str name: The name to assign to this output.
    :param Any value: The value of this output.
    """
    res = cast("Stack", get_root_resource())
    if known_types.is_stack(res):
        res.output(name, value)
    else:
        raise Exception(
            "Failed to export output. Root resource is not an instance of 'Stack'"
        )


def create_urn(
    name: "Input[str]",
    type_: "Input[str]",
    parent: Optional[Union["Resource", "Input[str]"]] = None,
    project: Optional[str] = None,
    stack: Optional[str] = None,
) -> "Output[str]":
    """
    create_urn computes a URN from the combination of a resource name, resource type, optional
    parent, optional project and optional stack.
    """
    return create_urn_internal(name, type_, parent, project, stack)


def _parse_resource_reference(ref: str) -> Tuple[str, str]:
    """
    Parses the URN and ID out of the provider reference.
    """
    last_sep = ref.rindex("::")
    ref_urn = ref[:last_sep]
    ref_id = ref[last_sep + 2 :]
    return (ref_urn, ref_id)
