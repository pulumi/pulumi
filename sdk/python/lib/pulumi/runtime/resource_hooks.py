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

import asyncio
from typing import Optional, Union
from ..resource_hooks import ResourceHookArgs, ResourceHook, ResourceHookBinding
from .proto import RegisterResourceRequest, ConstructRequest


def noop(args: ResourceHookArgs) -> None:
    pass


class StubResourceHook(ResourceHook):
    """
    StubResourceHook is a resource hook that does nothing.

    We need to reconstruct `:class:ResourceHook` instances to set on the
    `:class:ResourceOption`s, but we only have the name available to us. We also
    know that these hooks have already been registered, so we can construct
    dummy hooks here, that will later be serialized back into list of hook
    names.
    """

    def __init__(self, name: str):
        # Note: we intentionally do not call super here, because we do not
        # want to kick off a registration for this hook.
        self.name = name
        self.callback = noop
        self.opts = None
        self._registered = asyncio.Future()
        # Set the hook as registered.
        self._registered.set_result(None)


def _binding_from_proto(
    protoBinding: Optional[
        Union[
            RegisterResourceRequest.ResourceHooksBinding,
            ConstructRequest.ResourceHooksBinding,
        ]
    ],
) -> Optional[ResourceHookBinding]:
    """
    Convert a hook binding from a protobuf message to a Python object with `:class:StubHook`s.
    """
    if not protoBinding:
        return None
    return ResourceHookBinding(
        before_create=list(map(StubResourceHook, protoBinding.before_create)),
        after_create=list(map(StubResourceHook, protoBinding.after_create)),
        before_update=list(map(StubResourceHook, protoBinding.before_update)),
        after_update=list(map(StubResourceHook, protoBinding.after_update)),
        before_delete=list(map(StubResourceHook, protoBinding.before_delete)),
        after_delete=list(map(StubResourceHook, protoBinding.after_delete)),
    )
