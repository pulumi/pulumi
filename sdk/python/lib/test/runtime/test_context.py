# Copyright 2026, Pulumi Corporation.
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
from contextvars import ContextVar
import threading

import pytest

from pulumi.runtime._context import wrap_with_context


@pytest.mark.asyncio
async def test_wrap_with_context_propagates_contextvars():
    value = ContextVar("test_context_value", default="default")
    value.set("caller")
    caller_thread = threading.get_ident()

    worker_thread, worker_value = await asyncio.get_running_loop().run_in_executor(
        None,
        wrap_with_context(lambda: (threading.get_ident(), value.get())),
    )

    assert worker_thread != caller_thread
    assert worker_value == "caller"
