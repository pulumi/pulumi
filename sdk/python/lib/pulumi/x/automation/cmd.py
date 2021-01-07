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

import os
import subprocess
from typing import List, Mapping, Optional, Callable, Any

from .errors import create_command_error

OnOutput = Callable[[str], Any]


class CommandResult:
    stdout: str
    stderr: str
    code: int

    def __init__(self, stdout: str, stderr: str, code: int) -> None:
        self.stdout = stdout
        self.stderr = stderr
        self.code = code

    def __str__(self) -> str:
        return f"\n code: {self.code}\n stdout: {self.stdout}\n stderr: {self.stderr}"


def _run_pulumi_cmd(args: List[str],
                    cwd: str,
                    additional_env: Mapping[str, str],
                    on_output: Optional[OnOutput] = None) -> CommandResult:
    # All commands should be run in non-interactive mode.
    # This causes commands to fail rather than prompting for input (and thus hanging indefinitely).
    args.append("--non-interactive")
    env = os.environ.copy().update(additional_env)
    cmd = ["pulumi"]
    cmd.extend(args)

    stdout_chunks: List[str] = []
    stderr_chunks: List[str] = []

    # TODO: figure out why this won't stream output
    with subprocess.Popen(cmd,
                          stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE,
                          encoding="utf-8",
                          cwd=cwd,
                          env=env) as process:
        while True:
            output = process.stdout.readline()
            err = process.stderr.readline()
            if output == "" and err == "" and process.poll() is not None:
                code = process.poll()
                break
            if output:
                if on_output:
                    on_output(output.strip())
                stdout_chunks.append(output.strip())
            if err:
                stderr_chunks.append(err.strip())

    result = CommandResult(stderr=''.join(stderr_chunks), stdout=''.join(stdout_chunks), code=code)
    if code != 0:
        raise create_command_error(result)

    return result
