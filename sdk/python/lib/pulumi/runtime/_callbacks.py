# Copyright 2016-2024, Pulumi Corporation.
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

from __future__ import annotations

from concurrent import futures
from typing import TYPE_CHECKING, Awaitable, Callable, Dict, List, Mapping, Union, cast
import uuid

import grpc

from google.protobuf.message import Message

from .. import log
from .proto import (
    alias_pb2,
    callback_pb2,
    callback_pb2_grpc,
    resource_pb2,
    resource_pb2_grpc,
)
from .rpc import deserialize_properties, serialize_properties
from .sync_await import _sync_await

if TYPE_CHECKING:
    from ..resource import Alias, ResourceOptions, ResourceTransform


# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
_GRPC_CHANNEL_OPTIONS = [("grpc.max_receive_message_length", _MAX_RPC_MESSAGE_SIZE)]


_CallbackFunction = Callable[[bytes], Awaitable[Message]]


class _CallbackServicer(callback_pb2_grpc.CallbacksServicer):
    _servicers: List[_CallbackServicer] = []

    _callbacks: Dict[str, _CallbackFunction]
    _monitor: resource_pb2_grpc.ResourceMonitorStub
    _server: grpc.Server
    _target: str

    _transforms: Dict[ResourceTransform, str]

    def __init__(self, monitor: resource_pb2_grpc.ResourceMonitorStub):
        log.debug("Creating CallbackServicer")
        _CallbackServicer._servicers.append(self)
        self._callbacks = {}
        self._transforms = {}
        self._monitor = monitor
        self._server = grpc.server(
            futures.ThreadPoolExecutor(
                max_workers=4
            ),  # pylint: disable=consider-using-with
            options=_GRPC_CHANNEL_OPTIONS,
        )
        callback_pb2_grpc.add_CallbacksServicer_to_server(self, self._server)
        port = self._server.add_insecure_port(address="127.0.0.1:0")
        self._server.start()
        self._target = f"127.0.0.1:{port}"

    @classmethod
    def shutdown(cls):
        for servicer in cls._servicers:
            servicer._server.stop(grace=0)

    def Invoke(
        self, request: callback_pb2.CallbackInvokeRequest, context
    ) -> callback_pb2.CallbackInvokeResponse:
        log.debug(f"Invoke callback {request.token}")
        callback = self._callbacks.get(request.token)
        if callback is None:
            context.abort(
                grpc.StatusCode.INVALID_ARGUMENT,
                f"Callback with token {request.token} not found!",
            )
            raise Exception("Callback not found!")

        response = _sync_await(callback(request.request))
        return callback_pb2.CallbackInvokeResponse(
            response=response.SerializeToString()
        )

    def register_transform(self, transform: ResourceTransform) -> callback_pb2.Callback:
        # If this transform function has already been registered, return it.
        token = self._transforms.get(transform)
        if token is not None:
            return callback_pb2.Callback(token=token, target=self._target)

        from ..resource import (  # pylint: disable=import-outside-toplevel
            ResourceTransformArgs,
        )

        async def cb(s: bytes) -> Message:
            request: resource_pb2.TransformRequest = (
                resource_pb2.TransformRequest.FromString(s)
            )

            args = ResourceTransformArgs(
                custom=request.custom,
                type_=request.type,
                name=request.name,
                props=deserialize_properties(request.properties),
                opts=self._resource_options(request),
            )

            result = transform(args)

            if result is None:
                return resource_pb2.TransformResponse(
                    properties=request.properties,
                    options=request.options,
                )

            result_props = await serialize_properties(result.props, {})

            result_opts = (
                await self._transformation_resource_options(result.opts)
                if result.opts is not None
                else None
            )

            return resource_pb2.TransformResponse(
                properties=result_props,
                options=result_opts,
            )

        token = str(uuid.uuid4())
        self._callbacks[token] = cb
        self._transforms[transform] = token
        return callback_pb2.Callback(
            token=token,
            target=self._target,
        )

    def register_stack_transform(self, transform: ResourceTransform):
        callback = self.register_transform(transform)
        try:
            self._monitor.RegisterStackTransform(callback)
        except:
            # Remove the transform since we didn't manage to actually register it.
            self._transforms.pop(transform)
            self._callbacks.pop(callback.token)
            raise

    def _resource_options(
        self, request: resource_pb2.TransformRequest
    ) -> ResourceOptions:
        from ..resource import (  # pylint: disable=import-outside-toplevel
            CustomTimeouts,
            DependencyProviderResource,
            DependencyResource,
            ResourceOptions,
        )

        opts = (
            request.options
            if request.HasField("options")
            else resource_pb2.TransformResourceOptions()
        )

        ropts = ResourceOptions()

        if opts.HasField("delete_before_replace"):
            ropts.delete_before_replace = opts.delete_before_replace

        additional_secret_outputs = list(opts.additional_secret_outputs)
        if additional_secret_outputs:
            ropts.additional_secret_outputs = additional_secret_outputs

        providers = {
            k: DependencyProviderResource(v) for k, v in opts.providers.items()
        }
        if providers:
            ropts.providers = providers

        aliases = [self._alias(a) for a in opts.aliases]
        if aliases:
            ropts.aliases = aliases

        if opts.HasField("custom_timeouts"):
            custom_timeouts = CustomTimeouts()
            if opts.custom_timeouts.create:
                custom_timeouts.create = opts.custom_timeouts.create
            if opts.custom_timeouts.update:
                custom_timeouts.update = opts.custom_timeouts.update
            if opts.custom_timeouts.delete:
                custom_timeouts.delete = opts.custom_timeouts.delete
            ropts.custom_timeouts = custom_timeouts

        if opts.deleted_with:
            ropts.deleted_with = DependencyResource(opts.deleted_with)

        depends_on = [DependencyResource(d) for d in opts.depends_on]
        if depends_on:
            ropts.depends_on = depends_on

        ignore_changes = list(opts.ignore_changes)
        if ignore_changes:
            ropts.ignore_changes = ignore_changes

        if request.parent:
            ropts.parent = DependencyResource(request.parent)

        if opts.plugin_download_url:
            ropts.plugin_download_url = opts.plugin_download_url

        if opts.protect:
            ropts.protect = opts.protect

        if opts.provider:
            ropts.provider = DependencyProviderResource(opts.provider)

        replace_on_changes = list(opts.replace_on_changes)
        if replace_on_changes:
            ropts.replace_on_changes = replace_on_changes

        if opts.retain_on_delete:
            ropts.retain_on_delete = opts.retain_on_delete

        if opts.version:
            ropts.version = opts.version

        return ropts

    def _alias(self, alias: alias_pb2.Alias) -> Union[str, Alias]:
        if alias.HasField("urn"):
            return alias.urn

        # pylint: disable=import-outside-toplevel
        from ..resource import Alias

        a = Alias()
        if alias.spec.name:
            a.name = alias.spec.name
        if alias.spec.type:
            a.type_ = alias.spec.type
        if alias.spec.project:
            a.project = alias.spec.project
        if alias.spec.stack:
            a.stack = alias.spec.stack
        if alias.spec.parentUrn:
            a.parent = alias.spec.parentUrn
        elif alias.spec.noParent:
            a.parent = None
        return a

    async def _transformation_resource_options(
        self, opts: ResourceOptions
    ) -> resource_pb2.TransformResourceOptions:
        from .resource import (  # pylint: disable=import-outside-toplevel
            _create_custom_timeouts,
            _create_provider_ref,
            _resolve_depends_on_urns,
            create_alias_spec,
        )
        from ..resource import (  # pylint: disable=import-outside-toplevel
            Alias,
            ProviderResource,
            _collapse_providers,
        )
        from ..output import Output  # pylint: disable=import-outside-toplevel

        aliases: List[alias_pb2.Alias] = []
        if opts.aliases:
            for alias in opts.aliases:
                resolved = await Output.from_input(alias).future()
                if resolved is None:
                    continue
                if isinstance(resolved, str):
                    aliases.append(alias_pb2.Alias(urn=resolved))
                else:
                    spec = await create_alias_spec(cast(Alias, resolved))
                    aliases.append(alias_pb2.Alias(spec=spec))

        custom_timeouts = None
        if opts.custom_timeouts is not None:
            custom_timeouts = _create_custom_timeouts(opts.custom_timeouts)

        depends_on = await _resolve_depends_on_urns(opts)

        ignore_changes = None
        if opts.ignore_changes:
            ignore_changes = list(opts.ignore_changes)

        replace_on_changes = None
        if opts.replace_on_changes:
            replace_on_changes = list(opts.replace_on_changes)

        additional_secret_outputs = None
        if opts.additional_secret_outputs:
            additional_secret_outputs = list(opts.additional_secret_outputs)

        result = resource_pb2.TransformResourceOptions(
            aliases=aliases or None,
            custom_timeouts=custom_timeouts,
            depends_on=depends_on or None,
            ignore_changes=ignore_changes,
            replace_on_changes=replace_on_changes,
            additional_secret_outputs=additional_secret_outputs,
        )

        if opts.deleted_with is not None:
            result.deleted_with = cast(str, await opts.deleted_with.urn.future())
        if opts.plugin_download_url:
            result.plugin_download_url = opts.plugin_download_url
        if opts.protect:
            result.protect = opts.protect
        if opts.retain_on_delete:
            result.retain_on_delete = opts.retain_on_delete
        if opts.version:
            result.version = opts.version
        if opts.provider is not None:
            result.provider = await _create_provider_ref(opts.provider)
        if opts.delete_before_replace:
            result.delete_before_replace = opts.delete_before_replace
        if opts.providers:
            _collapse_providers(opts)
            providers = cast(Mapping[str, ProviderResource], opts.providers)
            result.providers.update(
                {k: await _create_provider_ref(v) for k, v in providers.items()}
            )

        return result
