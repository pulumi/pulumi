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

from pulumi.automation import LocalWorkspace, Stack, InlineProgramArgs
from .test_utils import get_test_stack


class TestConfigFile(unittest.IsolatedAsyncioTestCase):
    async def test_config_file_option(self):
        """Tests that the config_file option is correctly passed to the operations"""
        # Create a temporary config file for testing
        with tempfile.NamedTemporaryFile(
            suffix=".yaml", delete=False
        ) as temp_config_file:
            temp_config_file.write(b"config:\n  test_project:test_key: test_value")
            config_file_path = temp_config_file.name

        # Create a simple program for testing
        def pulumi_program():
            return {"result": "success"}

        try:
            # Create a stack with the temporary config file
            stack_name = get_test_stack()
            stack_args = InlineProgramArgs(
                stack_name=stack_name,
                project_name="test_project",
                program=pulumi_program,
            )
            stack = await Stack.create(stack_args, None)

            # Test preview with config_file
            preview_stdout = StringIO()
            await stack.preview(
                on_output=lambda text: preview_stdout.write(text),
                config_file=config_file_path,
            )
            self.assertIn("--config-file", preview_stdout.getvalue())

            # Test update with config_file
            up_stdout = StringIO()
            await stack.up(
                on_output=lambda text: up_stdout.write(text),
                config_file=config_file_path,
            )
            self.assertIn("--config-file", up_stdout.getvalue())

            # Test refresh with config_file
            refresh_stdout = StringIO()
            await stack.refresh(
                on_output=lambda text: refresh_stdout.write(text),
                config_file=config_file_path,
            )
            self.assertIn("--config-file", refresh_stdout.getvalue())

            # Test destroy with config_file
            destroy_stdout = StringIO()
            await stack.destroy(
                on_output=lambda text: destroy_stdout.write(text),
                config_file=config_file_path,
            )
            self.assertIn("--config-file", destroy_stdout.getvalue())

            # Clean up
            await LocalWorkspace().remove_stack(stack_name)
        finally:
            # Remove temporary config file
            os.unlink(config_file_path)
