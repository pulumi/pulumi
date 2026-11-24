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

import asyncio
import sys
import traceback

import pulumi
import pytest
from pulumi.runtime import stack
from pulumi.runtime import mocks, settings
from pulumi.runtime.settings import _get_rpc_manager, get_root_resource


class RuntimeMocks(pulumi.runtime.Mocks):
    def call(self, args):
        return {}

    def new_resource(self, args):
        return f"{args.name}-id", args.inputs


def configure_mock_runtime():
    settings.reset_options(project="project", stack="stack")
    settings.SETTINGS.monitor = mocks.MockMonitor(RuntimeMocks())
    settings.SETTINGS.engine = mocks.MockEngine(None)


@pytest.fixture(autouse=True)
def reset_runtime_settings():
    configure_mock_runtime()
    yield
    settings.reset_options(project="project", stack="stack")


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


@pytest.mark.asyncio
async def test_run_in_stack_preserves_synchronous_programs():
    def program():
        pulumi.export("message", "hello")

    await stack.run_in_stack(program)

    root = get_root_resource()
    assert isinstance(root, stack.Stack)
    assert root.outputs == {"message": "hello"}


@pytest.mark.asyncio
async def test_run_in_stack_natively_awaits_inline_async_programs():
    async def program():
        await asyncio.sleep(0)
        pulumi.export("message", "hello")

    await stack.run_in_stack(program)

    root = get_root_resource()
    assert isinstance(root, stack.Stack)
    assert root.outputs == {"message": "hello"}


@pytest.mark.asyncio
async def test_run_returns_stack_outputs_and_creates_resources():
    calls = 0

    async def value():
        await asyncio.sleep(0)
        return "resolved"

    async def program():
        nonlocal calls
        calls += 1

        before = pulumi.ComponentResource("test:index:Component", "before")
        await asyncio.sleep(0)
        after = pulumi.ComponentResource("test:index:Component", "after")

        return {
            "plain": "value",
            "output": pulumi.Output.from_input(value()),
            "secret": pulumi.Output.secret("sensitive"),
            "beforeUrn": before.urn,
            "afterUrn": after.urn,
        }

    def load_program():
        pulumi.run(program)

    await stack.run_in_stack(load_program)

    root = get_root_resource()
    assert isinstance(root, stack.Stack)
    assert calls == 1
    assert root.outputs["plain"] == "value"
    assert await root.outputs["output"].future() == "resolved"
    assert await root.outputs["secret"].future() == "sensitive"
    assert await root.outputs["secret"].is_secret()
    assert await root.outputs["beforeUrn"].future() == (
        "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:index:Component::before"
    )
    assert await root.outputs["afterUrn"].future() == (
        "urn:pulumi:stack::project::pulumi:pulumi:Stack$test:index:Component::after"
    )


@pytest.mark.asyncio
async def test_run_accepts_none_with_explicit_exports():
    async def program():
        await asyncio.sleep(0)
        pulumi.export("message", "hello")

    def load_program():
        pulumi.run(program)

    await stack.run_in_stack(load_program)

    root = get_root_resource()
    assert isinstance(root, stack.Stack)
    assert root.outputs == {"message": "hello"}


def test_run_requires_active_stack():
    async def program():
        pass

    with pytest.raises(
        pulumi.RunError,
        match="may only be called while a Pulumi program is running",
    ):
        pulumi.run(program)


@pytest.mark.asyncio
async def test_run_rejects_multiple_calls():
    calls = []

    async def first():
        calls.append("first")

    async def second():
        calls.append("second")

    def load_program():
        pulumi.run(first)
        with pytest.raises(
            pulumi.RunError,
            match="may only be called once",
        ):
            pulumi.run(second)

    await stack.run_in_stack(load_program)

    assert calls == ["first"]


@pytest.mark.asyncio
async def test_run_rejects_nested_calls():
    calls = []

    async def nested():
        calls.append("nested")

    async def program():
        calls.append("program")
        pulumi.run(nested)

    def load_program():
        pulumi.run(program)

    with pytest.raises(
        pulumi.RunError,
        match="may only be called once",
    ):
        await stack.run_in_stack(load_program)

    assert calls == ["program"]


@pytest.mark.asyncio
async def test_run_registration_does_not_leak_between_runs():
    async def run_program(message):
        async def program():
            return {"message": message}

        def load_program():
            pulumi.run(program)

        await stack.run_in_stack(load_program)
        root = get_root_resource()
        assert isinstance(root, stack.Stack)
        return root.outputs

    assert await run_program("first") == {"message": "first"}

    configure_mock_runtime()

    assert await run_program("second") == {"message": "second"}


@pytest.mark.asyncio
async def test_run_rejects_non_awaitable_callback_result():
    def program():
        return {"message": "hello"}

    def load_program():
        pulumi.run(program)  # type: ignore[arg-type]

    with pytest.raises(TypeError, match="must return an awaitable"):
        await stack.run_in_stack(load_program)


@pytest.mark.asyncio
async def test_run_merges_returned_outputs_via_export():
    async def program():
        return {
            "message": "from return",
            "returned": "returned value",
        }

    def load_program():
        pulumi.export("existing", "existing value")
        pulumi.export("message", "from export")
        pulumi.run(program)

    await stack.run_in_stack(load_program)

    root = get_root_resource()
    assert isinstance(root, stack.Stack)
    assert root.outputs == {
        "existing": "existing value",
        "message": "from return",
        "returned": "returned value",
    }


@pytest.mark.asyncio
async def test_run_propagates_exception_after_await():
    async def program():
        await asyncio.sleep(0)
        raise RuntimeError("entrypoint failed")

    def load_program():
        pulumi.run(program)

    with pytest.raises(RuntimeError, match="entrypoint failed"):
        await stack.run_in_stack(load_program)


@pytest.mark.asyncio
async def test_stack_finish_registers_outputs_once(monkeypatch):
    registrations = []

    monkeypatch.setattr(
        stack.Stack,
        "register_outputs",
        lambda _self, outputs: registrations.append(outputs),
    )

    async def program():
        return {"message": "hello"}

    def load_program():
        pulumi.run(program)

    await stack.run_in_stack(load_program)

    root = get_root_resource()
    assert isinstance(root, stack.Stack)
    root._finish()
    assert registrations == [{"message": "hello"}]


@pytest.mark.asyncio
async def test_run_pulumi_func_shuts_down_callbacks_after_cancellation(monkeypatch):
    cleaned_up = False

    async def shutdown_callbacks():
        nonlocal cleaned_up
        cleaned_up = True

    async def program():
        raise asyncio.CancelledError

    monkeypatch.setattr(stack, "_shutdown_callbacks", shutdown_callbacks)

    with pytest.raises(asyncio.CancelledError):
        await stack.run_pulumi_func(program)

    assert cleaned_up
