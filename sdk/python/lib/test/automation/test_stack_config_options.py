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

import unittest
from unittest.mock import patch, MagicMock
import tempfile

from pulumi.automation import (
    Stack,
    LocalWorkspace,
    ProjectSettings,
    ConfigValue,
    ConfigOptions,
    GetAllConfigOptions,
    CommandError,
)


class TestStackConfigOptions(unittest.TestCase):
    """Test coverage for ConfigOptions error handling paths in Stack class."""

    def setUp(self):
        self.project_name = "test_project"
        self.stack_name = "test_stack"
        self.project_settings = ProjectSettings(self.project_name, runtime="python")

    def test_get_config_with_options_non_path_error(self):
        """Test that exceptions unrelated to path are properly raised."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "get_config") as mock_get_config:
                mock_get_config.side_effect = PermissionError("Access denied")

                stack = Stack.create_or_select(self.stack_name, ws)
                options = ConfigOptions(path=False)

                # This should raise the original PermissionError, not catch it
                with self.assertRaises(PermissionError) as cm:
                    stack.get_config_with_options("key", options)

                self.assertEqual(str(cm.exception), "Access denied")

    def test_get_config_with_options_no_path_attribute(self):
        """Test handling when options object has no path attribute."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        # Create a custom options object without path attribute
        class CustomOptions:
            def __init__(self):
                self.config_file = "custom.yaml"
                # Deliberately omit path attribute

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "get_config") as mock_get_config:
                # Should raise exception and not retry since no path attribute
                mock_get_config.side_effect = ValueError("Invalid config")

                stack = Stack.create_or_select(self.stack_name, ws)
                options = CustomOptions()

                with self.assertRaises(ValueError):
                    stack.get_config_with_options("key", options)

                # Should only be called once (no retry)
                mock_get_config.assert_called_once()

    def test_set_config_with_options_path_true_non_path_error(self):
        """Test when path=True but error is unrelated to path support."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "set_config") as mock_set_config:
                # First call with path fails, but not due to path parameter
                # Second call without path succeeds
                mock_set_config.side_effect = [
                    ValueError("Invalid config value format"),
                    None,  # Second call succeeds
                ]

                stack = Stack.create_or_select(self.stack_name, ws)
                options = ConfigOptions(path=True)

                # Should retry without path and succeed
                stack.set_config_with_options("key", ConfigValue("value"), options)

                # Verify it was called twice
                self.assertEqual(mock_set_config.call_count, 2)

                # Check the arguments for both calls
                first_call_args = mock_set_config.call_args_list[0]
                second_call_args = mock_set_config.call_args_list[1]

                # First call should have path=True
                self.assertTrue(first_call_args.kwargs.get("path"))
                # Second call should have path omitted or False
                self.assertNotIn("path", second_call_args.kwargs)

    def test_remove_config_with_options_path_false_with_error(self):
        """Test when path=False but operation still fails."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "remove_config") as mock_remove_config:
                mock_remove_config.side_effect = FileNotFoundError(
                    "Config file not found"
                )

                stack = Stack.create_or_select(self.stack_name, ws)
                options = ConfigOptions(path=False, config_file="missing.yaml")

                # Should raise the original error since path=False
                with self.assertRaises(FileNotFoundError) as cm:
                    stack.remove_config_with_options("key", options)

                self.assertEqual(str(cm.exception), "Config file not found")
                # Should only be called once (no retry)
                mock_remove_config.assert_called_once()

    def test_get_all_config_with_options_retry_fails(self):
        """Test when both the original call and retry fail."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "get_all_config") as mock_get_all_config:
                # Both calls fail
                mock_get_all_config.side_effect = [
                    CommandError("--path not supported"),
                    CommandError("Config file corrupted"),
                ]

                stack = Stack.create_or_select(self.stack_name, ws)
                options = GetAllConfigOptions(path=True, show_secrets=True)

                # Should raise the second exception
                with self.assertRaises(CommandError) as cm:
                    stack.get_all_config_with_options(options)

                self.assertEqual(str(cm.exception), "Config file corrupted")
                self.assertEqual(mock_get_all_config.call_count, 2)

    def test_set_all_config_with_options_error_handling(self):
        """Test error handling for set_all_config_with_options."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "set_all_config") as mock_set_all_config:
                # Simulate path not supported error
                mock_set_all_config.side_effect = [
                    CommandError("Unknown flag: --path"),
                    None,  # Retry succeeds
                ]

                stack = Stack.create_or_select(self.stack_name, ws)
                config = {"key": ConfigValue("value")}
                options = ConfigOptions(path=True, config_file="test.yaml")

                # Should retry without path but keep config_file
                stack.set_all_config_with_options(config, options)

                # Verify retry preserved config_file option
                second_call_kwargs = mock_set_all_config.call_args_list[1].kwargs
                self.assertEqual(second_call_kwargs.get("config_file"), "test.yaml")
                self.assertNotIn("path", second_call_kwargs)

    def test_remove_all_config_with_options_none_options(self):
        """Test remove_all_config_with_options with None options."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "remove_all_config") as mock_remove_all_config:
                mock_remove_all_config.return_value = (
                    None  # Simulate successful removal
                )

                with patch.object(
                    Stack, "remove_all_config"
                ) as mock_stack_remove_all_config:
                    mock_stack_remove_all_config.return_value = None

                    stack = Stack.create_or_select(self.stack_name, ws)
                    keys = ["key1", "key2"]

                    # Should call regular remove_all_config
                    stack.remove_all_config_with_options(keys, None)

                    # Should call the stack's remove_all_config method
                    mock_stack_remove_all_config.assert_called_once_with(keys)

    def test_get_all_config_with_options_show_secrets_preserved(self):
        """Test that show_secrets is preserved during retry."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "get_all_config") as mock_get_all_config:
                # First call fails due to path, second succeeds
                mock_get_all_config.side_effect = [
                    CommandError("Unknown flag: --path"),
                    {"key": ConfigValue("secret_value", secret=True)},
                ]

                stack = Stack.create_or_select(self.stack_name, ws)
                options = GetAllConfigOptions(
                    path=True, show_secrets=True, config_file="test.yaml"
                )

                result = stack.get_all_config_with_options(options)

                # Verify second call preserved show_secrets and config_file
                second_call_kwargs = mock_get_all_config.call_args_list[1].kwargs
                self.assertTrue(second_call_kwargs.get("show_secrets"))
                self.assertEqual(second_call_kwargs.get("config_file"), "test.yaml")
                self.assertNotIn("path", second_call_kwargs)

    def test_config_with_options_exception_without_path_attribute(self):
        """Test exception handling when options has path=False."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "set_config") as mock_set_config:
                # Exception occurs even though path=False
                mock_set_config.side_effect = IOError("Disk full")

                stack = Stack.create_or_select(self.stack_name, ws)
                options = ConfigOptions(path=False)

                # Should raise original error without retry
                with self.assertRaises(IOError) as cm:
                    stack.set_config_with_options("key", ConfigValue("value"), options)

                self.assertEqual(str(cm.exception), "Disk full")
                mock_set_config.assert_called_once()

    def test_get_config_with_options_with_custom_exception(self):
        """Test handling of custom exceptions."""
        ws = LocalWorkspace(project_settings=self.project_settings)

        class CustomError(Exception):
            pass

        with patch.object(ws, "create_stack"), patch.object(ws, "select_stack"):
            with patch.object(ws, "get_config") as mock_get_config:
                mock_get_config.side_effect = CustomError("Custom error")

                stack = Stack.create_or_select(self.stack_name, ws)
                options = ConfigOptions(path=True)

                # Should attempt retry but still fail with custom error
                with self.assertRaises(CustomError):
                    stack.get_config_with_options("key", options)

                # Should be called twice (original + retry)
                self.assertEqual(mock_get_config.call_count, 2)
