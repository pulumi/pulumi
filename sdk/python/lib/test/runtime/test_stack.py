# Copyright 2023, Pulumi Corporation.
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

import sys
import traceback

import pytest
from pulumi.runtime import stack
from pulumi.runtime.settings import _get_rpc_manager


async def _explode(n=10):
    if n == 0:
        raise Exception("sadness")
    await _explode(n - 1)


@pytest.mark.asyncio
async def test_wait_for_rpcs():
    """
    Verifies that the exception produced by wait_for_rpcs
    reproduces the original stack trace
    rather than wait_for_rpcs's own stack trace.
    """

    await _get_rpc_manager().do_rpc("sadness", _explode)()
    try:
        await stack.wait_for_rpcs()
    except Exception:
        tb = "".join(traceback.format_tb(sys.exc_info()[2]))
    else:
        pytest.fail("Expected Exception")
        tb = ""

    assert "await _explode(n - 1)" in tb
