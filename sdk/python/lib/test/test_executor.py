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
import textwrap
from pathlib import Path

import pytest


def run_executor(tmp_path: Path, program: str) -> subprocess.CompletedProcess[str]:
    main = tmp_path / "__main__.py"
    main.write_text(textwrap.dedent(program))
    executor = Path(__file__).resolve().parents[2] / "cmd/pulumi-language-python-exec"
    return subprocess.run(
        [
            sys.executable,
            str(executor),
            "--project",
            "project",
            "--stack",
            "stack",
            "--parallel",
            "1",
            str(main),
        ],
        capture_output=True,
        text=True,
        timeout=20,
    )


@pytest.mark.parametrize(
    "program",
    [
        """
        import signal
        signal.raise_signal(signal.SIGINT)
        """,
        """
        import signal
        try:
            raise RuntimeError("resource monitor shut down")
        except RuntimeError:
            signal.raise_signal(signal.SIGINT)
        """,
        """
        import signal
        from concurrent.futures import ThreadPoolExecutor

        def shutdown(self, wait=True, **kwargs):
            signal.raise_signal(signal.SIGINT)

        ThreadPoolExecutor.shutdown = shutdown
        """,
    ],
    ids=["program", "handling-error", "executor-shutdown"],
)
def test_executor_interrupt_does_not_print_traceback(tmp_path: Path, program: str):
    result = run_executor(tmp_path, program)
    assert result.returncode != 0
    assert "Traceback" not in result.stdout + result.stderr


def test_executor_reports_program_exceptions(tmp_path: Path):
    result = run_executor(tmp_path, 'raise ValueError("application failure")')
    assert result.returncode == 32
    assert "ValueError: application failure" in result.stdout + result.stderr
    assert "__main__.py" in result.stdout + result.stderr


def test_executor_reports_cleanup_exceptions(tmp_path: Path):
    result = run_executor(
        tmp_path,
        """
        from concurrent.futures import ThreadPoolExecutor

        def shutdown(self, wait=True, **kwargs):
            raise ValueError("cleanup failure")

        ThreadPoolExecutor.shutdown = shutdown
        """,
    )
    assert result.returncode != 0
    assert "ValueError: cleanup failure" in result.stdout + result.stderr


def test_executor_preserves_program_interrupt_handler(tmp_path: Path):
    result = run_executor(
        tmp_path,
        """
        import signal
        signal.signal(signal.SIGINT, lambda *_: print("handled interrupt"))
        signal.raise_signal(signal.SIGINT)
        """,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    assert "handled interrupt" in result.stdout
