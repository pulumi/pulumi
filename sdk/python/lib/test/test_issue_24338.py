# Copyright 2016, Pulumi Corporation.
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

import pulumi
from pulumi.runtime import mocks


class NoLoopMocks(mocks.Mocks):
    def new_resource(self, args: mocks.MockResourceArgs):
        return f"{args.name}_id", args.inputs

    def call(self, args: mocks.MockCallArgs):
        return {}, None


def test_set_mocks_without_event_loop():
    """
    Verify that set_mocks works when called with no running event loop.

    This reproduces the scenario reported in #24338: Python 3.14 no longer
    creates an implicit event loop, so asyncio.Future() in resource_output()
    raises RuntimeError when reached synchronously from set_mocks().

    The fix ensures resource_output() creates a new event loop when none is
    running, so set_mocks() succeeds at module scope.
    """
    saved = asyncio.get_event_loop()

    try:
        # Remove the event loop from the current thread to simulate the
        # module-scope set_mocks() call on Python 3.14.
        asyncio.set_event_loop(None)

        # This must not raise RuntimeError even when no loop is running.
        pulumi.runtime.set_mocks(NoLoopMocks())
    finally:
        asyncio.set_event_loop(saved)


def test_set_mocks_with_existing_loop_not_affected():
    """
    Verify that the fix does not clobber or interfere with an existing
    running event loop (the normal case).
    """
    saved = asyncio.get_event_loop()

    try:
        pulumi.runtime.set_mocks(NoLoopMocks())
        # The existing loop must still be the current one.
        assert asyncio.get_event_loop() is saved
    finally:
        asyncio.set_event_loop(saved)
