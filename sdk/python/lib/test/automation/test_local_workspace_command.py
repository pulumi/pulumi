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

import tempfile
import unittest
from typing import Optional
from unittest.mock import MagicMock

from semver import VersionInfo

from pulumi.automation import LocalWorkspace, PulumiCommand
from pulumi.automation._cmd import CommandResult
from pulumi.automation._stack import Stack, StackInitMode


def _mock_pulumi_command(
    recorded_args: list[list[str]],
    stdout: str = "",
) -> PulumiCommand:
    """
    Build a ``PulumiCommand`` whose ``run`` records the args it was called with
    and returns a ``CommandResult`` with the provided ``stdout`` so workspace
    and stack wiring can be exercised without spawning an actual subprocess.
    """

    mock = MagicMock(spec=PulumiCommand)
    mock.command = "pulumi"
    mock.version = VersionInfo.parse("3.200.0")

    def run(
        args: list[str],
        cwd: str,
        additional_env,
        on_output: Optional[object] = None,
        on_error: Optional[object] = None,
    ) -> CommandResult:
        recorded_args.append(list(args))
        return CommandResult(stdout=stdout, stderr="", code=0)

    mock.run.side_effect = run
    return mock


class TestLocalWorkspaceCommand(unittest.TestCase):
    def test_cancel_invokes_generated_cli_api(self):
        """
        ``stack.cancel()`` should go through the auto-generated CLI interface
        and end up calling ``PulumiCommand.run`` with the equivalent of the
        previous hand-written invocation: ``pulumi cancel --yes --stack <name>``.
        """
        recorded: list[list[str]] = []
        pulumi_command = _mock_pulumi_command(recorded)

        with tempfile.TemporaryDirectory(prefix="automation-test-") as work_dir:
            workspace = LocalWorkspace(work_dir=work_dir, pulumi_command=pulumi_command)
            # Use SELECT mode so the Stack constructor only issues a `stack
            # select` against the mocked CLI rather than trying to create one.
            stack = Stack("cancel-test", workspace, StackInitMode.SELECT)

            # Drop the `stack select` call recorded during construction so the
            # assertion below can focus on what `cancel()` itself emits.
            recorded.clear()

            stack.cancel()

        self.assertEqual(len(recorded), 1, "expected cancel to invoke the CLI once")
        self.assertEqual(
            recorded[0],
            ["cancel", "--yes", "--stack", "cancel-test"],
        )

    def test_org_get_default_invokes_generated_cli_api(self):
        """
        ``workspace.org_get_default()`` should delegate to the generated API
        and return the CLI stdout stripped of surrounding whitespace.
        """
        recorded: list[list[str]] = []
        pulumi_command = _mock_pulumi_command(recorded, stdout="my-org\n")

        with tempfile.TemporaryDirectory(prefix="automation-test-") as work_dir:
            workspace = LocalWorkspace(work_dir=work_dir, pulumi_command=pulumi_command)
            result = workspace.org_get_default()

        self.assertEqual(result, "my-org")
        self.assertEqual(len(recorded), 1)
        self.assertEqual(recorded[0], ["org", "get-default"])

    def test_org_set_default_invokes_generated_cli_api(self):
        """
        ``workspace.org_set_default(name)`` should delegate to the generated API
        and pass the organization name as a positional argument.
        """
        recorded: list[list[str]] = []
        pulumi_command = _mock_pulumi_command(recorded)

        with tempfile.TemporaryDirectory(prefix="automation-test-") as work_dir:
            workspace = LocalWorkspace(work_dir=work_dir, pulumi_command=pulumi_command)
            workspace.org_set_default("my-org")

        self.assertEqual(len(recorded), 1)
        self.assertEqual(recorded[0], ["org", "set-default", "--", "my-org"])


if __name__ == "__main__":
    unittest.main()
