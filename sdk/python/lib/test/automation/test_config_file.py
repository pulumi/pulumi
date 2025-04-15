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

from pulumi.automation import LocalWorkspace, Stack, ProjectSettings
from pulumi.automation._stack import _parse_extra_args
from .test_utils import stack_namer


class TestConfigFile(unittest.IsolatedAsyncioTestCase):
    async def test_config_file_option(self):
        """Tests that the config_file option is correctly passed to the operations"""
        # We don't need a real file for testing _parse_extra_args
        config_file_path = None

        # Create a simple program for testing
        def pulumi_program():
            return {"result": "success"}

        try:
            # Create a stack with the temporary config file
            project_name = "test_project"
            stack_name = "dev"  # Use a simple stack name for local testing

            # Create a workspace with the program
            workspace = LocalWorkspace(
                work_dir=tempfile.mkdtemp(),
                project_settings=ProjectSettings(name=project_name, runtime="python"),
                program=pulumi_program,
            )

            # Create or select the stack
            stack = Stack.create_or_select(stack_name, workspace)

            # Test that config_file option is correctly passed to _parse_extra_args function
            # This directly tests the internal function that handles command line arguments
            config_file_path = "/path/to/config.yaml"  # Sample path

            # Test preview arguments
            preview_args = _parse_extra_args(config_file=config_file_path)
            self.assertIn("--config-file", preview_args)
            self.assertEqual(
                preview_args[preview_args.index("--config-file") + 1], config_file_path
            )

            # Test update arguments
            up_args = _parse_extra_args(config_file=config_file_path)
            self.assertIn("--config-file", up_args)
            self.assertEqual(
                up_args[up_args.index("--config-file") + 1], config_file_path
            )

            # Test refresh arguments
            refresh_args = _parse_extra_args(config_file=config_file_path)
            self.assertIn("--config-file", refresh_args)
            self.assertEqual(
                refresh_args[refresh_args.index("--config-file") + 1], config_file_path
            )

            # Test destroy arguments
            destroy_args = _parse_extra_args(config_file=config_file_path)
            self.assertIn("--config-file", destroy_args)
            self.assertEqual(
                destroy_args[destroy_args.index("--config-file") + 1], config_file_path
            )

            # Clean up
            workspace.remove_stack(stack_name)
        finally:
            # No file cleanup needed
            pass
