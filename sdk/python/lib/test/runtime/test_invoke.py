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


from pulumi.runtime.invoke import _extract_single_value
import pytest


def test_extract_single_value():
    d = {"foo": "bar"}
    v = _extract_single_value(d)
    assert v == "bar"


def test_nondicts_return_identity():
    x = 123
    y = _extract_single_value(x)
    assert x is y


def test_empty_dicts_throw():
    with pytest.raises(IndexError):
        _extract_single_value({})


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "variant", ["invoke", "invoke_single", "invoke_output", "invoke_output_single"]
)
@pytest.mark.parametrize("parent_handle", ["", "caller-stack"])
async def test_invoke_captures_initiation_context(variant, parent_handle):
    import inspect
    from pathlib import Path

    from pulumi import runtime
    from pulumi.runtime._registration_context import _registration_context
    from pulumi.runtime.mocks import MockMonitor, Mocks
    from pulumi.runtime.settings import Settings, configure

    class InvokeMocks(Mocks):
        def new_resource(self, args):
            return args.name, args.inputs

        def call(self, args):
            return {"value": "ok"}

    class RecordingMonitor(MockMonitor):
        def Invoke(self, request):
            requests.append(request)
            return super().Invoke(request)

    requests = []
    configure(Settings("project", "stack", monitor=RecordingMonitor(InvokeMocks())))
    invoke = getattr(runtime, variant)

    def sdk_wrapper():
        return invoke("test:index:lookup", {})

    with _registration_context(parent_handle):
        line = inspect.currentframe().f_lineno + 1
        result = sdk_wrapper()
    if "output" in variant:
        await result.future()

    assert len(requests) == 1
    request = requests[0]
    assert request.sourcePosition.uri == Path(__file__).as_uri()
    assert request.sourcePosition.line == line
    assert request.sourcePosition in [frame.pc for frame in request.stackTrace.frames]
    assert request.stackTrace.frames[0].origin == "pulumi"
    assert any(
        frame.module == "pulumi.runtime.invoke" for frame in request.stackTrace.frames
    )
    assert request.parentStackTraceHandle == parent_handle
