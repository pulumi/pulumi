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

import os
import signal
import time

import pytest


def test_automation_api_in_forked_worker():
    """
    Test that Pulumi Automation API works in a forked process.
    Regression test for https://github.com/pulumi/pulumi/issues/21934
    """

    import pulumi  # import before fork

    read_fd, write_fd = os.pipe()
    pid = os.fork()

    if pid == 0:  # Child process
        os.close(read_fd)

        try:
            from pulumi import automation as auto

            stack = auto.create_or_select_stack(
                stack_name="test",
                project_name="test",
                program=lambda: None,
                opts=auto.LocalWorkspaceOptions(
                    project_settings=auto.ProjectSettings(
                        name="test",
                        runtime="python",
                        backend=auto.ProjectBackend(url="file://~"),
                    ),
                    env_vars={"PULUMI_CONFIG_PASSPHRASE": ""},
                ),
            )
            stack.preview(on_output=lambda x: None)

            os.write(write_fd, b"SUCCESS")
            os.close(write_fd)
            os._exit(0)
        except Exception as e:
            error_msg = f"ERROR: {type(e).__name__}: {str(e)}"
            os.write(write_fd, error_msg.encode())
            os.close(write_fd)
            os._exit(1)

    else:  # Parent process
        os.close(write_fd)

        timeout = 120
        start = time.time()
        status = None
        while time.time() - start < timeout:
            pid_result, status = os.waitpid(pid, os.WNOHANG)
            if pid_result != 0:
                break
            time.sleep(0.1)
        else:
            os.kill(pid, signal.SIGKILL)
            _, status = os.waitpid(pid, 0)
            pytest.fail(f"Child process did not complete within {timeout}s")

        result = b""
        while chunk := os.read(read_fd, 1024):
            result += chunk
        os.close(read_fd)

        if os.WIFSIGNALED(status):
            sig = os.WTERMSIG(status)
            sig_name = (
                signal.Signals(sig).name
                if sig in signal.Signals._value2member_map_
                else str(sig)
            )
            pytest.fail(f"Child crashed with {sig_name}")

        if os.WIFEXITED(status) and (code := os.WEXITSTATUS(status)) != 0:
            result_str = result.decode() if result else "No error message"
            pytest.fail(f"Child exited with code {code}: {result_str}")

        assert result == b"SUCCESS"
