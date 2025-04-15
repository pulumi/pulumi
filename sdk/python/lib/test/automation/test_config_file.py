# Copyright 2024-2024, Pulumi Corporation.
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
import tempfile
from io import StringIO
import unittest

from pulumi.automation import LocalWorkspace, Stack, ProjectSettings, OpType
from pulumi.automation._stack import _parse_extra_args
from .test_utils import stack_namer


class TestConfigFile(unittest.IsolatedAsyncioTestCase):
    async def test_config_file_option(self):
        """Tests that the config_file option is correctly passed to the operations"""
        config_file_path = os.path.join(
            os.path.dirname(__file__), "data", "yaml", "Pulumi.local.yaml"
        )

        def pulumi_program():
            from pulumi import Config, export

            config = Config()
            export("plain", config.get("plain"))

        try:
            project_name = "test_project"
            stack_name = "dev"

            workspace = LocalWorkspace(
                work_dir=tempfile.mkdtemp(),
                project_settings=ProjectSettings(name=project_name, runtime="python"),
                program=pulumi_program,
            )

            stack = Stack.create_or_select(stack_name, workspace)

            # We will test the config file option for both _parse_extra_args and the stack.preview, stack.up, stack.refresh, stack.destroy methods
            extra_args = _parse_extra_args(config_file=config_file_path)
            self.assertIn("--config-file", extra_args)
            self.assertEqual(
                extra_args[extra_args.index("--config-file") + 1], config_file_path
            )

            self.assertEqual(
                stack.up(config_file=config_file_path).outputs["plain"].value, "plain"
            )

            self.assertEqual(
                stack.preview(config_file=config_file_path).change_summary.get(
                    OpType.SAME
                ),
                1,
            )

            self.assertEqual(
                stack.refresh(config_file=config_file_path).summary.result, "succeeded"
            )

            self.assertEqual(
                stack.destroy(config_file=config_file_path).summary.result, "succeeded"
            )

        finally:
            workspace.remove_stack(stack_name, force=True)
