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
import asyncio
import os
import pathlib
import traceback
from typing import (
    TYPE_CHECKING,
    Any,
    Callable,
    Dict,
    Iterable,
    List,
    Mapping,
    NamedTuple,
    Optional,
    Sequence,
    Set,
    Tuple,
    Union,
)

import grpc
from google.protobuf import struct_pb2

from .. import _types, log
from .. import urn as urn_util
from ..output import Input, Output
from ..runtime.proto import alias_pb2, resource_pb2, source_pb2, callback_pb2
from . import known_types, rpc, settings
from .rpc import _expand_dependencies
from .settings import (
    _get_callbacks,
    _get_rpc_manager,
    _sync_monitor_supports_transforms,
    handle_grpc_error,
)

if TYPE_CHECKING:
    from .. import (
        Alias,
        CustomResource,
        Inputs,
        ProviderResource,
        Resource,
        CustomTimeouts,
    )
    from ..resource import ResourceOptions


class ResourceResolverOperations(NamedTuple):
    """
    The set of properties resulting from a successful call to prepare_resource.
    """

    parent_urn: Optional[str]
    """
    This resource's parent URN.
    """

    serialized_props: struct_pb2.Struct
    """
    This resource's input properties, serialized into protobuf structures.
    """

    dependencies: Set[str]
    """
    The set of URNs, corresponding to the resources that this resource depends on.
    """

    provider_ref: Optional[str]
    """
    An optional reference to a provider that should be used for this resource's CRUD operations.
    """

    provider_refs: Dict[str, str]
    """
    An optional dict of references to providers that should be used for this resource's CRUD operations.
    """

    property_dependencies: Dict[str, List[str]]
    """
    A map from property name to the URNs of the resources the property depends on.
    """

    aliases: List[alias_pb2.Alias]
    """
    A list of aliases applied to this resource.
    """

    deleted_with_urn: Optional[str]
    """
    If set, the providers Delete method will not be called for this resource
    if specified resource is being deleted as well.
    """

    supports_alias_specs: bool
    """
    Returns whether the resource monitor supports alias specs which allows sending full alias specifications
    to the engine.
    """


async def prepare_aliases(
    resource: "Resource",
    resource_options: Optional["ResourceOptions"],
    supports_alias_specs: bool,
) -> List[alias_pb2.Alias]:
    aliases: List[alias_pb2.Alias] = []
    if resource_options is None or resource_options.aliases is None:
        return aliases

    if supports_alias_specs:
        for alias in resource_options.aliases:
            resolved_alias = await Output.from_input(alias).future()
            if resolved_alias is None:
                continue
            if isinstance(resolved_alias, str):
                aliases.append(alias_pb2.Alias(urn=resolved_alias))
            else:
                alias_spec = await create_alias_spec(resolved_alias)  # type: ignore
                aliases.append(alias_pb2.Alias(spec=alias_spec))
    else:
        # Using an version of the engine that does not support alias specs.  We will need to
        # compute the aliases ourselves as full URNs and sent them to the engine as such.
        alias_urns = all_aliases(
            resource_options.aliases,
            resource._name,
            resource._type,
            resource_options.parent,
        )

        distinct_alias_urns = set()
        for alias_urn in alias_urns:
            alias_urn_value = await Output.from_input(alias_urn).future()
            if (
                alias_urn_value is not None
                and alias_urn_value not in distinct_alias_urns
            ):
                distinct_alias_urns.add(alias_urn_value)

        for alias_urn in distinct_alias_urns:
            aliases.append(alias_pb2.Alias(urn=alias_urn))

    return aliases


async def _create_provider_ref(provider: "ProviderResource") -> str:
    # Wait for the provider to resolve and construct a provider reference from it.
    # A provider reference is a well-known string (two ::-separated values) that the engine interprets.
    urn = await provider.urn.future()
    pid = await provider.id.future() or rpc.UNKNOWN
    return f"{urn}::{pid}"


# Prepares for an RPC that will manufacture a resource, and hence deals with input and output properties.
# pylint: disable=too-many-locals
async def prepare_resource(
    res: "Resource",
    ty: str,
    custom: bool,
    remote: bool,
    props: "Inputs",
    opts: Optional["ResourceOptions"],
    typ: Optional[type] = None,
) -> ResourceResolverOperations:
    # Before we can proceed, all our dependencies must be finished.
    explicit_urn_dependencies: Set[str] = set()
    if opts is not None and opts.depends_on is not None:
        explicit_urn_dependencies = await _resolve_depends_on_urns(
            opts, from_resource=res
        )

    # Serialize out all our props to their final values.  In doing so, we'll also collect all
    # the Resources pointed to by any Dependency objects we encounter, adding them to 'implicit_dependencies'.
    property_dependencies_resources: Dict[str, List["Resource"]] = {}

    # If we have type information, we'll use it for translations rather than the resource's translate_input_property.
    translate: Optional[Callable[[str], str]] = res.translate_input_property
    if typ is not None:
        translate = None

    # To initially scope the use of this new feature, we only keep output values when
    # remote is true (for multi-lang components).
    serialized_props = await rpc.serialize_properties(
        props,
        property_dependencies_resources,
        res,
        translate,
        typ,
        keep_output_values=remote,
    )

    # Wait for our parent to resolve
    parent_urn: Optional[str] = ""
    if opts is not None and opts.parent is not None:
        parent_urn = await opts.parent.urn.future()
    # TODO(sean) is it necessary to check the type here?
    elif ty != "pulumi:pulumi:Stack":
        # If no parent was provided, parent to the root resource.
        parent = settings.get_root_resource()
        if parent is not None:
            parent_urn = await parent.urn.future()

    # Construct the provider reference, if we were given a provider to use.
    provider_ref = None
    send_provider = custom
    if remote and opts is not None and opts.provider is not None:
        # If it's a remote component and a provider was specified, only
        # send the provider in the request if the provider's package is
        # the same as the component's package.
        pkg = _pkg_from_type(ty)
        if pkg is not None and pkg == opts.provider.package:
            send_provider = True
    if send_provider and opts is not None and opts.provider is not None:
        provider_ref = await _create_provider_ref(opts.provider)

    # For remote resources, merge any provider opts into a single dict, and then create a new dict with all of the
    # resolved provider refs.
    provider_refs: Dict[str, str] = {}
    if (remote or not custom) and opts is not None:
        providers = convert_providers(opts.provider, opts.providers)
        for name, provider in providers.items():
            provider_refs[name] = await _create_provider_ref(provider)

    dependencies: Set[str] = set(explicit_urn_dependencies)
    property_dependencies: Dict[str, List[str]] = {}
    for key, deps in property_dependencies_resources.items():
        urns = await _expand_dependencies(deps, from_resource=res)
        dependencies |= urns
        property_dependencies[key] = list(urns)

    supports_alias_specs = await settings.monitor_supports_alias_specs()
    aliases = await prepare_aliases(res, opts, supports_alias_specs)
    deleted_with_urn: Optional[str] = ""
    if opts is not None and opts.deleted_with is not None:
        deleted_with_urn = await opts.deleted_with.urn.future()

    return ResourceResolverOperations(
        parent_urn,
        serialized_props,
        dependencies,
        provider_ref,
        provider_refs,
        property_dependencies,
        aliases,
        deleted_with_urn,
        supports_alias_specs,
    )


async def create_alias_spec(resolved_alias: "Alias") -> alias_pb2.Alias.Spec:
    name: str = ""
    resource_type: str = ""
    stack: str = ""
    project: str = ""
    parent_urn: str = ""
    no_parent: bool = False

    if resolved_alias.name is not ... and resolved_alias.name is not None:
        name = resolved_alias.name

    if resolved_alias.type_ is not ... and resolved_alias.type_ is not None:
        resource_type = resolved_alias.type_

    if resolved_alias.stack is not ...:
        stack_value = await Output.from_input(resolved_alias.stack).future()
        if stack_value is not None:
            stack = stack_value

    if resolved_alias.project is not ...:
        project_value = await Output.from_input(resolved_alias.project).future()
        if project_value is not None:
            project = project_value

    if resolved_alias.parent is ...:
        # parent is not specified (e.g. Alias(name="Foo")),
        # default to current parent
        no_parent = False
    elif resolved_alias.parent is None:
        # parent is explicitly set to None (e.g. Alias(name="Foo", parent=None))
        # this means that the resource previously had no parent
        no_parent = True
    else:
        # pylint: disable-next=import-outside-toplevel
        from .. import Resource

        if isinstance(resolved_alias.parent, Resource):
            parent_urn_value = await resolved_alias.parent.urn.future()
            if parent_urn_value is not None:
                parent_urn = parent_urn_value
                no_parent = False
        elif isinstance(resolved_alias.parent, str):
            parent_urn = resolved_alias.parent
            no_parent = False
        else:
            # assume parent is Input[str] where str is the URN of the parent
            parent_urn_value = await Output.from_input(resolved_alias.parent).future()  # type: ignore
            if parent_urn_value is not None:
                parent_urn = parent_urn_value
                no_parent = False

    if no_parent:
        return alias_pb2.Alias.Spec(
            name=name,
            type=resource_type,
            stack=stack,
            project=project,
            noParent=no_parent,
        )

    return alias_pb2.Alias.Spec(
        name=name,
        type=resource_type,
        stack=stack,
        project=project,
        parentUrn=parent_urn,
    )


def inherited_child_alias(
    child_name: str, parent_name: str, parent_alias: "Input[str]", child_type: str
) -> "Output[str]":
    """
    inherited_child_alias computes the alias that should be applied to a child based on an alias
    applied to it's parent. This may involve changing the name of the resource in cases where the
    resource has a named derived from the name of the parent, and the parent name changed.
    """

    #   If the child name has the parent name as a prefix, then we make the assumption that it was
    #   constructed from the convention of using `{name}-details` as the name of the child resource.  To
    #   ensure this is aliased correctly, we must then also replace the parent aliases name in the prefix of
    #   the child resource name.
    #
    #   For example:
    #   * name: "newapp-function"
    #   * opts.parent.__name: "newapp"
    #   * parentAlias: "urn:pulumi:stackname::projectname::awsx:ec2:Vpc::app"
    #   * parentAliasName: "app"
    #   * aliasName: "app-function"
    #   * childAlias: "urn:pulumi:stackname::projectname::aws:s3/bucket:Bucket::app-function"
    alias_name = Output.from_input(child_name)
    if child_name.startswith(parent_name):
        alias_name = Output.from_input(parent_alias).apply(
            lambda u: u[u.rfind("::") + 2 :] + child_name[len(parent_name) :]
        )

    return create_urn(alias_name, child_type, parent_alias)


# Extract the type and name parts of a URN
def urn_type_and_name(urn: str) -> Tuple[str, str]:
    parts = urn.split("::")
    type_parts = parts[2].split("$")
    return (parts[3], type_parts[-1])


def all_aliases(
    child_aliases: Optional[Sequence["Input[Union[str, Alias]]"]],
    child_name: str,
    child_type: str,
    parent: Optional["Resource"],
) -> "List[Input[str]]":
    """
    Make a copy of the aliases array, and add to it any implicit aliases inherited from its parent.
    If there are N child aliases, and M parent aliases, there will be (M+1)*(N+1)-1 total aliases,
    or, as calculated in the logic below, N+(M*(1+N)).
    """
    aliases: "List[Input[str]]" = []

    for child_alias in child_aliases or []:
        aliases.append(
            collapse_alias_to_urn(child_alias, child_name, child_type, parent)
        )

    if parent is not None:
        parent_name = parent._name
        for parent_alias in parent._aliases:
            aliases.append(
                inherited_child_alias(
                    child_name, parent._name, parent_alias, child_type
                )
            )
            for child_alias in child_aliases or []:
                child_alias_urn = collapse_alias_to_urn(
                    child_alias, child_name, child_type, parent
                )

                def inherited_alias_for_child_urn(
                    child_alias_urn: str, parent_alias=parent_alias
                ) -> "Output[str]":
                    aliased_child_name, aliased_child_type = urn_type_and_name(
                        child_alias_urn
                    )
                    return inherited_child_alias(
                        aliased_child_name,
                        parent_name,
                        parent_alias,
                        aliased_child_type,
                    )

                inherited_alias: Output[str] = child_alias_urn.apply(
                    inherited_alias_for_child_urn
                )
                aliases.append(inherited_alias)

    return aliases


def collapse_alias_to_urn(
    alias: "Input[Union[Alias, str]]",
    defaultName: str,
    defaultType: str,
    defaultParent: Optional["Resource"],
) -> "Output[str]":
    """
    collapse_alias_to_urn turns an Alias into a URN given a set of default data
    """

    def collapse_alias_to_urn_worker(inner: "Union[Alias, str]") -> Output[str]:
        if isinstance(inner, str):
            return Output.from_input(inner)

        name: str = inner.name if inner.name is not ... else defaultName  # type: ignore
        type_: str = inner.type_ if inner.type_ is not ... else defaultType  # type: ignore
        parent = inner.parent if inner.parent is not ... else defaultParent  # type: ignore
        project: "Input[str]" = settings.get_project()
        if inner.project is not ... and inner.project is not None:
            project = inner.project
        stack: "Input[str]" = settings.get_stack()
        if inner.stack is not ... and inner.stack is not None:
            stack = inner.stack

        if name is None:
            raise Exception("No valid 'name' passed in for alias.")

        if type_ is None:
            raise Exception("No valid 'type_' passed in for alias.")

        all_args = [project, stack]
        return Output.all(*all_args).apply(
            lambda args: create_urn(name, type_, parent, args[0], args[1])
        )

    inputAlias: "Output[Union[Alias, str]]" = Output.from_input(alias)
    return inputAlias.apply(collapse_alias_to_urn_worker)


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
    parent_prefix: Optional[Output[str]] = None
    if parent is not None:
        parent_urn = None
        # pylint: disable=import-outside-toplevel
        from .. import Resource

        if isinstance(parent, Resource):
            parent_urn = parent.urn
        else:
            parent_urn = Output.from_input(parent)

        parent_prefix = parent_urn.apply(lambda u: u[0 : u.rfind("::")] + "$")
    else:
        if stack is None:
            stack = settings.get_stack()

        if project is None:
            project = settings.get_project()

        parent_prefix = Output.from_input("urn:pulumi:" + stack + "::" + project + "::")

    all_args = [parent_prefix, type_, name]
    # invariant http://mypy.readthedocs.io/en/latest/common_issues.html#variance
    return Output.all(*all_args).apply(lambda arr: arr[0] + arr[1] + "::" + arr[2])  # type: ignore


def resource_output(
    res: "Resource",
) -> Tuple[Callable[[Any, bool, bool, Optional[Exception]], None], "Output"]:
    value_future: asyncio.Future[Any] = asyncio.Future()
    known_future: asyncio.Future[bool] = asyncio.Future()
    secret_future: asyncio.Future[bool] = asyncio.Future()

    def resolve(value: Any, known: bool, secret: bool, exn: Optional[Exception]):
        if exn is not None:
            value_future.set_exception(exn)
            known_future.set_exception(exn)
            secret_future.set_exception(exn)
        else:
            value_future.set_result(value)
            known_future.set_result(known)
            secret_future.set_result(secret)

    return resolve, Output({res}, value_future, known_future, secret_future)


def get_resource(
    res: "Resource", props: "Inputs", custom: bool, urn: str, typ: Optional[type] = None
) -> None:
    log.debug(f"getting resource: urn={urn}")

    # If we have type information, we'll use its and the resource's type/name metadata
    # for name translations rather than using the resource's translation methods.
    transform_using_type_metadata = typ is not None

    # Extract the resource type from the URN.
    urn_parts = urn_util._parse_urn(urn)
    ty = urn_parts.typ

    # Initialize the URN property on the resource.
    (resolve_urn, res.__dict__["urn"]) = resource_output(res)

    # If this is a custom resource, initialize its ID property.
    resolve_id: Optional[Callable[[Any, bool, bool, Optional[Exception]], None]] = None
    if custom:
        (resolve_id, res.__dict__["id"]) = resource_output(res)

    # Like the other resource functions, "transfer" all input properties onto unresolved futures on res.
    resolvers = rpc.transfer_properties(res, props, custom)

    async def do_get():
        try:
            resolver = await prepare_resource(res, ty, custom, False, props, None, typ)

            monitor = settings.get_monitor()
            inputs = await rpc.serialize_properties({"urn": urn}, {})

            accept_resources = not (
                os.getenv("PULUMI_DISABLE_RESOURCE_REFERENCES", "").upper()
                in {"TRUE", "1"}
            )
            req = resource_pb2.ResourceInvokeRequest(
                tok="pulumi:pulumi:getResource",
                args=inputs,
                provider="",
                version="",
                acceptResources=accept_resources,
            )

            def do_invoke():
                try:
                    return monitor.Invoke(req)
                except grpc.RpcError as exn:
                    handle_grpc_error(exn)
                    return None

            resp = await asyncio.get_event_loop().run_in_executor(None, do_invoke)

            # If the invoke failed, raise an error.
            if resp.failures:
                raise Exception(
                    f"getResource failed: {resp.failures[0].reason} ({resp.failures[0].property})"
                )

        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}"
            )
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn(None, True, False, exn)
            if resolve_id is not None:
                resolve_id(None, True, False, exn)
            raise

        # Otherwise, grab the URN, ID, and output properties and resolve all of them.
        resp = getattr(resp, "return")

        log.debug(f"getResource completed successfully: ty={ty}, urn={resp['urn']}")
        resolve_urn(resp["urn"], True, False, None)
        if resolve_id:
            # The ID is known if (and only if) it is a non-empty string. If it's either None or an
            # empty string, we should treat it as unknown. TFBridge in particular is known to send
            # the empty string as an ID when doing a preview.
            is_known = bool(resp["id"])
            resolve_id(resp["id"], is_known, False, None)

        rpc.resolve_outputs(
            res,
            resolver.serialized_props,
            resp["state"],
            {},
            resolvers,
            custom,
            transform_using_type_metadata,
        )

    asyncio.ensure_future(_get_rpc_manager().do_rpc("get resource", do_get)())


def _translate_ignore_changes(
    res: "Resource", typ: Optional[type], ignore_changes: Optional[List[str]]
) -> Optional[List[str]]:
    if ignore_changes is not None:
        if typ is not None:
            # If `typ` is specified, use its type/name metadata for translation.
            input_names = _types.input_type_py_to_pulumi_names(typ)
            ignore_changes = list(
                map(lambda k: input_names.get(k) or k, ignore_changes)
            )
        elif res.translate_input_property is not None:
            ignore_changes = list(map(res.translate_input_property, ignore_changes))
    return ignore_changes


def _translate_additional_secret_outputs(
    res: "Resource", typ: Optional[type], additional_secret_outputs: Optional[List[str]]
) -> Optional[List[str]]:
    if additional_secret_outputs is not None:
        if typ is not None:
            # If a `typ` is specified, we've opt-ed in to doing translations using type/name metadata rather
            # than using the resource's tranlate_input_property. Use the resource's metadata to translate.
            output_names = _types.resource_py_to_pulumi_names(type(res))
            additional_secret_outputs = list(
                map(lambda k: output_names.get(k) or k, additional_secret_outputs)
            )
        elif res.translate_input_property is not None:
            # Note that while `additional_secret_outputs` lists property names that are outputs, we
            # call `translate_input_property` because it is the method that converts from the
            # language projection name to the provider name, which is what we want.
            additional_secret_outputs = list(
                map(res.translate_input_property, additional_secret_outputs)
            )
    return additional_secret_outputs


def _translate_replace_on_changes(
    res: "Resource", typ: Optional[type], replace_on_changes: Optional[List[str]]
) -> Optional[List[str]]:
    if replace_on_changes is not None:
        if typ is not None:
            # If `typ` is specified, use its type/name metadata for translation.
            input_names = _types.input_type_py_to_pulumi_names(typ)
            replace_on_changes = list(
                map(lambda k: input_names.get(k) or k, replace_on_changes)
            )
        elif res.translate_input_property is not None:
            replace_on_changes = list(
                map(res.translate_input_property, replace_on_changes)
            )
    return replace_on_changes


def _get_source_position(skip: int) -> Optional[source_pb2.SourcePosition]:
    """
    Returns the source position of the Nth stack frame, where N is skip+1.

    This is used to compute the source position of the user code that instantiated a resource. The number of frames to
    skip is parameterized in order to account for differing call stacks for different operations.
    """

    # Capture a stack that includes the Nth stack frame. If the stack is not deep enough, return the empty string.
    stack = traceback.extract_stack(limit=skip + 2)
    if len(stack) < skip + 2:
        return None

    # Extract the Nth stack frame. If that frame is missing file or line information, return the empty string.
    caller = stack[0]
    if caller.filename == "" or caller.lineno is None:
        return None

    try:
        uri = pathlib.Path(caller.filename).as_uri()
    except BaseException:
        return None

    # Convert the Nth source position to a source position URI by converting the filename to a URI and appending
    # the line and column fragment.
    return source_pb2.SourcePosition(uri=uri, line=caller.lineno)


def read_resource(
    res: "CustomResource",
    ty: str,
    name: str,
    props: "Inputs",
    opts: "ResourceOptions",
    typ: Optional[type] = None,
) -> None:
    if opts.id is None:
        raise Exception("Cannot read resource whose options are lacking an ID value")

    log.debug(f"reading resource: ty={ty}, name={name}, id={opts.id}")
    monitor = settings.get_monitor()

    # If we have type information, we'll use its and the resource's type/name metadata
    # for name translations rather than using the resource's translation methods.
    transform_using_type_metadata = typ is not None

    # Prepare the resource, similar to a RegisterResource. Reads are deliberately similar to RegisterResource except
    # that we are populating the Resource object with properties associated with an already-live resource.
    #
    # Same as below, we initialize the URN property on the resource, which will always be resolved.
    (resolve_urn, res.__dict__["urn"]) = resource_output(res)

    # Furthermore, since resources being Read must always be custom resources (enforced in the
    # Resource constructor), we'll need to set up the ID field which will be populated at the end of
    # the ReadResource call.
    #
    # Note that we technically already have the ID (opts.id), but it's more consistent with the rest
    # of the model to resolve it asynchronously along with all of the other resources.

    (resolve_id, res.__dict__["id"]) = resource_output(res)

    # Like below, "transfer" all input properties onto unresolved futures on res.
    custom = True  # Reads are always for custom resources (non-components)
    resolvers = rpc.transfer_properties(res, props, custom)

    # Get the source position.
    #
    # This is somewhat brittle in that it expects a call stack of the form:
    # - read_resource
    # - Resource class constructor
    # - abstract Resource subclass constructor
    # - concrete Resource subclass constructor
    # - user code
    #
    # This stack reflects the expected class hierarchy of "cloud resource / component resource < customresource/componentresource < resource".
    source_position = _get_source_position(4)

    async def do_read():
        try:
            resolver = await prepare_resource(res, ty, True, False, props, opts, typ)

            # Resolve the ID that we were given. Note that we are explicitly discarding the list of
            # dependencies returned to us from "serialize_property" (the second argument). This is
            # because a "read" resource does not actually have any dependencies at all in the cloud
            # provider sense, because a read resource already exists. We do not need to track this
            # dependency.
            resolved_id = await rpc.serialize_property(opts.id, [], None)
            log.debug(f"read prepared: ty={ty}, name={name}, id={opts.id}")

            # These inputs will end up in the snapshot, so if there are any additional secret
            # outputs, record them here.
            additional_secret_outputs = _translate_additional_secret_outputs(
                res, typ, opts.additional_secret_outputs
            )

            accept_resources = not (
                os.getenv("PULUMI_DISABLE_RESOURCE_REFERENCES", "").upper()
                in {"TRUE", "1"}
            )
            req = resource_pb2.ReadResourceRequest(
                type=ty,
                name=name,
                id=resolved_id,
                parent=resolver.parent_urn,
                provider=resolver.provider_ref,
                properties=resolver.serialized_props,
                dependencies=resolver.dependencies,
                version=opts.version or "",
                pluginDownloadURL=opts.plugin_download_url or "",
                acceptSecrets=True,
                acceptResources=accept_resources,
                additionalSecretOutputs=additional_secret_outputs,
                sourcePosition=source_position,
            )

            mock_urn = await create_urn(name, ty, resolver.parent_urn).future()

            def do_rpc_call():
                if monitor is None:
                    # If no monitor is available, we'll need to fake up a response, for testing.
                    return RegisterResponse(
                        mock_urn or "", None, resolver.serialized_props, None, None
                    )

                # If there is a monitor available, make the true RPC request to the engine.
                try:
                    return monitor.ReadResource(req)
                except grpc.RpcError as exn:
                    handle_grpc_error(exn)
                    return None

            resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)

        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}"
            )
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn(None, True, False, exn)
            resolve_id(None, True, False, exn)
            raise

        log.debug(f"resource read successful: ty={ty}, urn={resp.urn}")
        resolve_urn(resp.urn, True, False, None)
        resolve_id(resolved_id, True, False, None)  # Read IDs are always known.
        rpc.resolve_outputs(
            res,
            resolver.serialized_props,
            resp.properties,
            {},
            resolvers,
            custom,
            transform_using_type_metadata,
        )

    asyncio.ensure_future(_get_rpc_manager().do_rpc("read resource", do_read)())


def _create_custom_timeouts(
    custom_timeouts: "CustomTimeouts",
) -> "resource_pb2.RegisterResourceRequest.CustomTimeouts":
    result = resource_pb2.RegisterResourceRequest.CustomTimeouts()
    # It could be an actual CustomTimeouts object.
    if known_types.is_custom_timeouts(custom_timeouts):
        if custom_timeouts.create is not None:
            result.create = custom_timeouts.create
        if custom_timeouts.update is not None:
            result.update = custom_timeouts.update
        if custom_timeouts.delete is not None:
            result.delete = custom_timeouts.delete
    # Or, it could be a workaround passing in a dict.
    elif isinstance(custom_timeouts, dict):
        if "create" in custom_timeouts:
            result.create = custom_timeouts["create"]
        if "update" in custom_timeouts:
            result.update = custom_timeouts["update"]
        if "delete" in custom_timeouts:
            result.delete = custom_timeouts["delete"]
    else:
        raise Exception("Expected custom_timeouts to be a CustomTimeouts object")
    return result


def register_resource(
    res: "Resource",
    ty: str,
    name: str,
    custom: bool,
    remote: bool,
    new_dependency: Callable[[str], "Resource"],
    props: "Inputs",
    opts: Optional["ResourceOptions"],
    typ: Optional[type] = None,
) -> None:
    """
    Registers a new resource object with a given type t and name.  It returns the
    auto-generated URN and the ID that will resolve after the deployment has completed.  All
    properties will be initialized to property objects that the registration operation will resolve
    at the right time (or remain unresolved for deployments).
    """
    log.debug(
        f"registering resource: ty={ty}, name={name}, custom={custom}, remote={remote}"
    )
    monitor = settings.get_monitor()

    # If we have type information, we'll use its and the resource's type/name metadata
    # for name translations rather than using the resource's translation methods.
    transform_using_type_metadata = typ is not None

    # Prepare the resource.

    # Simply initialize the URN property and get prepared to resolve it later on.
    # Note: a resource urn will always get a value, and thus the output property
    # for it can always run .apply calls.
    (resolve_urn, res.__dict__["urn"]) = resource_output(res)

    # If a custom resource, make room for the ID property.
    resolve_id: Optional[Callable[[Any, bool, bool, Optional[Exception]], None]] = None
    if custom:
        (resolve_id, res.__dict__["id"]) = resource_output(res)

    # Now "transfer" all input properties into unresolved futures on res.  This way,
    # this resource will look like it has all its output properties to anyone it is
    # passed to.  However, those futures won't actually resolve until the RPC returns
    resolvers = rpc.transfer_properties(res, props, custom)

    # Get the source position.
    #
    # This is somewhat brittle in that it expects a call stack of the form:
    # - register_resource
    # - Resource class constructor
    # - abstract Resource subclass constructor
    # - concrete Resource subclass constructor
    # - user code
    #
    # This stack reflects the expected class hierarchy of "cloud resource / component resource < customresource/componentresource < resource".
    source_position = _get_source_position(4)

    async def do_register() -> None:
        try:
            from ..resource import (  # pylint: disable=import-outside-toplevel
                ResourceOptions,
            )

            nonlocal opts
            opts = opts if opts is not None else ResourceOptions()

            resolver = await prepare_resource(res, ty, custom, remote, props, opts, typ)
            log.debug(f"resource registration prepared: ty={ty}, name={name}")

            callbacks: List[callback_pb2.Callback] = []
            if opts.transforms:
                if not _sync_monitor_supports_transforms():
                    raise Exception(
                        "The Pulumi CLI does not support transforms. Please update the Pulumi CLI."
                    )
                callback_server = await _get_callbacks()
                if callback_server is None:
                    raise Exception("Callback server not initialized")
                for transform in opts.transforms:
                    callbacks.append(callback_server.register_transform(transform))

            property_dependencies = {}
            for key, deps in resolver.property_dependencies.items():
                property_dependencies[key] = (
                    resource_pb2.RegisterResourceRequest.PropertyDependencies(urns=deps)
                )

            ignore_changes = _translate_ignore_changes(res, typ, opts.ignore_changes)
            additional_secret_outputs = _translate_additional_secret_outputs(
                res, typ, opts.additional_secret_outputs
            )
            replace_on_changes = _translate_replace_on_changes(
                res, typ, opts.replace_on_changes
            )

            # Translate the CustomTimeouts object.
            custom_timeouts = None
            if opts.custom_timeouts is not None:
                custom_timeouts = _create_custom_timeouts(opts.custom_timeouts)

            if (
                resolver.deleted_with_urn
                and not await settings.monitor_supports_deleted_with()
            ):
                raise Exception(
                    "The Pulumi CLI does not support the DeletedWith option. Please update the Pulumi CLI."
                )

            accept_resources = not (
                os.getenv("PULUMI_DISABLE_RESOURCE_REFERENCES", "").upper()
                in {"TRUE", "1"}
            )

            full_aliases_specs: List[alias_pb2.Alias] | None = None
            alias_urns: List[str] | None = None
            if resolver.supports_alias_specs:
                full_aliases_specs = resolver.aliases
            else:
                alias_urns = [alias.urn for alias in resolver.aliases]

            req = resource_pb2.RegisterResourceRequest(
                type=ty,
                name=name,
                parent=resolver.parent_urn or "",
                custom=custom,
                object=resolver.serialized_props,
                protect=opts.protect or False,
                provider=resolver.provider_ref or "",
                providers=resolver.provider_refs,
                dependencies=resolver.dependencies,
                propertyDependencies=property_dependencies,
                deleteBeforeReplace=opts.delete_before_replace or False,
                deleteBeforeReplaceDefined=opts.delete_before_replace is not None,
                ignoreChanges=ignore_changes,
                version=opts.version or "",
                pluginDownloadURL=opts.plugin_download_url or "",
                acceptSecrets=True,
                acceptResources=accept_resources,
                additionalSecretOutputs=additional_secret_outputs,
                importId=opts.import_ or "",
                customTimeouts=custom_timeouts,
                aliases=full_aliases_specs,
                aliasURNs=alias_urns,
                supportsPartialValues=True,
                remote=remote,
                replaceOnChanges=replace_on_changes or [],
                retainOnDelete=opts.retain_on_delete or False,
                deletedWith=resolver.deleted_with_urn or "",
                sourcePosition=source_position,
                transforms=callbacks,
                supportsResultReporting=True,
            )

            mock_urn = await create_urn(name, ty, resolver.parent_urn).future()

            def do_rpc_call() -> (
                Optional[Union[RegisterResponse, resource_pb2.RegisterResourceResponse]]
            ):
                if monitor is None:
                    # If no monitor is available, we'll need to fake up a response, for testing.
                    return RegisterResponse(
                        mock_urn or "", None, resolver.serialized_props, None, None
                    )

                # If there is a monitor available, make the true RPC request to the engine.
                try:
                    return monitor.RegisterResource(req)
                except grpc.RpcError as exn:
                    handle_grpc_error(exn)
                    return None

            resp = await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        except Exception as exn:
            log.debug(
                f"exception when preparing or executing rpc: {traceback.format_exc()}"
            )
            rpc.resolve_outputs_due_to_exception(resolvers, exn)
            resolve_urn(None, True, False, exn)
            if resolve_id is not None:
                resolve_id(None, True, False, exn)
            raise

        if resp is None:
            return

        # At this point we would like to return successfully and call
        # `rpc.resolve_outputs`, but unfortunately that itself can
        # throw an exception sometimes. This was causing Pulumi
        # program to hang, so the additional try..except block is used
        # to propagate this exception into `rpc.resolve_outputs` which
        # causes it to display.

        resolve_outputs_called = False
        resolve_id_called = False
        resolve_urn_called = False

        try:
            log.debug(f"resource registration successful: ty={ty}, urn={resp.urn}")

            resolve_urn(resp.urn, True, False, None)
            resolve_urn_called = True

            if resolve_id is not None:
                # The ID is known if (and only if) it is a non-empty string. If it's either None or an
                # empty string, we should treat it as unknown. TFBridge in particular is known to send
                # the empty string as an ID when doing a preview.
                is_known = bool(resp.id)
                resolve_id(resp.id, is_known, False, None)
                resolve_id_called = True

            property_deps = {}
            rpc_deps = resp.propertyDependencies
            if rpc_deps:
                for k, v in rpc_deps.items():
                    urns = list(v.urns)
                    property_deps[k] = set(map(new_dependency, urns))

            keep_unknowns = resp.result == resource_pb2.Result.SUCCESS
            rpc.resolve_outputs(
                res,
                resolver.serialized_props,
                resp.object,
                property_deps,
                resolvers,
                custom,
                transform_using_type_metadata,
                keep_unknowns,
            )
            resolve_outputs_called = True

        except Exception as exn:
            log.debug(f"exception after executing rpc: {traceback.format_exc()}")

            if not resolve_outputs_called:
                rpc.resolve_outputs_due_to_exception(resolvers, exn)

            if not resolve_urn_called:
                resolve_urn(None, True, False, exn)

            if resolve_id is not None and not resolve_id_called:
                resolve_id(None, True, False, exn)

            raise

    asyncio.ensure_future(_get_rpc_manager().do_rpc("register resource", do_register)())


def register_resource_outputs(
    res: "Resource", outputs: "Union[Inputs, Output[Inputs]]"
):
    async def do_register_resource_outputs():
        urn = await res.urn.future()
        # serialize_properties expects a collection (empty is fine) but not None, but this is called pretty
        # much directly by users who could pass None in (although the type hints say they shouldn't).
        serialized_props = await rpc.serialize_properties(outputs or {}, {})
        log.debug(
            f"register resource outputs prepared: urn={urn}, props={serialized_props}"
        )
        monitor = settings.get_monitor()
        req = resource_pb2.RegisterResourceOutputsRequest(
            urn=urn, outputs=serialized_props
        )

        def do_rpc_call():
            if monitor is None:
                # If there's no engine attached, simply ignore it.
                return None

            try:
                return monitor.RegisterResourceOutputs(req)
            except grpc.RpcError as exn:
                handle_grpc_error(exn)
                return None

        await asyncio.get_event_loop().run_in_executor(None, do_rpc_call)
        log.debug(
            f"resource registration successful: urn={urn}, props={serialized_props}"
        )

    asyncio.ensure_future(
        _get_rpc_manager().do_rpc(
            "register resource outputs", do_register_resource_outputs
        )()
    )


class PropertyDependencies:
    urns: List[str]

    def __init__(self, urns: List[str]):
        self.urns = urns


class RegisterResponse:
    urn: str
    id: Optional[str]
    object: struct_pb2.Struct
    propertyDependencies: Optional[Dict[str, PropertyDependencies]]
    result: Optional[resource_pb2.Result.ValueType]

    # pylint: disable=redefined-builtin
    def __init__(
        self,
        urn: str,
        id: Optional[str],
        object: struct_pb2.Struct,
        propertyDependencies: Optional[Dict[str, PropertyDependencies]],
        result: Optional[resource_pb2.Result.ValueType],
    ):
        self.urn = urn
        self.id = id
        self.object = object
        self.propertyDependencies = propertyDependencies
        self.result = result


def convert_providers(
    provider: Optional["ProviderResource"],
    providers: Optional[
        Union[Mapping[str, "ProviderResource"], Sequence["ProviderResource"]]
    ],
) -> Mapping[str, "ProviderResource"]:
    """
    Merge all providers opts (opts.provider and both list and dict forms of opts.providers) into a single dict.
    """
    if provider is not None:
        return convert_providers(None, [provider])

    if providers is None:
        return {}

    if isinstance(providers, Mapping):
        return providers

    result = {}
    for p in providers:
        result[p.package] = p

    return result


async def _resolve_depends_on_urns(
    options: "ResourceOptions",
    from_resource: Optional["Resource"] = None,
) -> Set[str]:
    """
    Resolves the set of all dependent resources implied by
    `depends_on`, either directly listed or implied in the Input
    layer. Returns a deduplicated URN list.
    """

    if options.depends_on is None:
        return set()

    outer = Output._from_input_shallow(options._depends_on_list())
    all_deps = await outer.resources()
    inner_list = await outer.future() or []

    for i in inner_list:
        inner = Output.from_input(i)
        more_deps = await inner.resources()
        all_deps = all_deps | more_deps
        direct_dep = await inner.future()
        if direct_dep is not None:
            all_deps.add(direct_dep)

    return await rpc._expand_dependencies(all_deps, from_resource)


def _pkg_from_type(ty: str) -> Optional[str]:
    """
    Extract the pkg from the type token of the form "pkg:module:member".
    """
    parts = ty.split(":")
    if len(parts) != 3:
        return None
    return parts[0]
