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

import subprocess
import sys

import pytest

from pulumi.dynamic import Resource, ResourceProvider


def test_import_without_dill():
    subprocess.run(
        [
            sys.executable,
            "-c",
            "import sys; sys.modules['dill'] = None; import pulumi; import pulumi.dynamic",
        ],
        check=True,
        timeout=30,
    )


def test_dynamic_resource_without_dill(monkeypatch):
    monkeypatch.setitem(sys.modules, "dill", None)
    with pytest.raises(RuntimeError, match="Use Python 3.14 or earlier"):
        Resource(ResourceProvider(), "test", {})


def test_dynamic_provider_process_without_dill():
    result = subprocess.run(
        [
            sys.executable,
            "-c",
            "import sys, runpy; sys.modules['dill'] = None; "
            "runpy.run_module('pulumi.dynamic', run_name='__main__')",
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode != 0
    assert "Use Python 3.14 or earlier" in result.stderr
