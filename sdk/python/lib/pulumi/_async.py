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

"""Internal Async IO utilities, compat with Python 3.6."""

import asyncio
from typing import Any, Callable, TypeVar, cast


_F = TypeVar('_F', bound=Callable[..., Any])


def _asynchronized(func: _F) -> _F:
    """Decorates a function to acquire and release a lock.

    This makes sure that only one invocation of a function is active
    on the current event loop at one time. Since this is an asyncio
    lock, no real threads are blocked; only the invoking coroutine may
    be blocked.

    Usage:

        class MyClass:

            @_asynchronized
            async def my_func(self, x, y=None):
                ...

    """
    lock = asyncio.Lock()

    async def sync_func(*args, **kw):
        await lock.acquire()
        try:
            return await func(*args, **kw)
        finally:
            lock.release()

    return cast(_F, sync_func)
