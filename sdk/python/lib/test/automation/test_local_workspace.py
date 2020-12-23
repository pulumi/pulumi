# Copyright 2016-2020, Pulumi Corporation.
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
import unittest
from random import random
from pulumi.x.automation import LocalWorkspace, PluginInfo, ProjectSettings, StackSummary
from typing import List, Optional

extensions = ["json", "yaml", "yml"]


def test_path(*paths):
    return os.path.join(os.path.dirname(os.path.abspath(__file__)), *paths)


def get_test_suffix() -> int:
    return int(100000 + random() * 900000)


def found_plugin(plugin_list: List[PluginInfo], name: str, version: str) -> bool:
    for plugin in plugin_list:
        if plugin.name == name and plugin.version == version:
            return True
    return False


def get_stack(stack_list: List[StackSummary], name: str) -> Optional[StackSummary]:
    for stack in stack_list:
        if stack.name == name:
            return stack
    return None


class TestLocalWorkspace(unittest.TestCase):
    def test_project_settings(self):
        for ext in extensions:
            ws = LocalWorkspace(work_dir=test_path("data", ext))
            settings = ws.project_settings()
            self.assertEqual(settings.name, "testproj")
            self.assertEqual(settings.runtime, "go")
            self.assertEqual(settings.description, "A minimal Go Pulumi program")

    def test_stack_settings(self):
        for ext in extensions:
            ws = LocalWorkspace(work_dir=test_path("data", ext))
            settings = ws.stack_settings("dev")
            self.assertEqual(settings.secrets_provider, "abc")
            self.assertEqual(settings.config["plain"], "plain")
            self.assertEqual(settings.config["secure"].secure, "secret")

    def test_plugin_functions(self):
        ws = LocalWorkspace()
        # Install aws 3.0.0 plugin
        ws.install_plugin("aws", "v3.0.0")
        # Check the plugin is present
        plugin_list = ws.list_plugins()
        self.assertTrue(found_plugin(plugin_list, "aws", "3.0.0"))

        # Remove the plugin
        ws.remove_plugin("aws", "3.0.0")
        # Check that the plugin has been removed
        plugin_list = ws.list_plugins()
        self.assertFalse(found_plugin(plugin_list, "aws", "3.0.0"))

    def test_stack_functions(self):
        project_settings = ProjectSettings(name="python_test", runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = f"python_int_test_{get_test_suffix()}"
        second_stack_name = f"python_int_test_second_{get_test_suffix()}"

        # Create the stack
        ws.create_stack(stack_name)
        # Check the stack exists
        stacks = ws.list_stacks()
        first_stack = get_stack(stacks, stack_name)
        self.assertIsNotNone(first_stack)
        # Check that it's current
        self.assertTrue(first_stack.current)

        # Create another stack
        ws.create_stack(second_stack_name)
        # Check the second stack exists
        stacks = ws.list_stacks()
        first_stack = get_stack(stacks, stack_name)
        second_stack = get_stack(stacks, second_stack_name)
        self.assertIsNotNone(second_stack)
        # Check that second stack is current but the first is not
        self.assertFalse(first_stack.current)
        self.assertTrue(second_stack.current)

        # Select the first stack again
        ws.select_stack(stack_name)
        # Check the first stack is now current
        stacks = ws.list_stacks()
        first_stack = get_stack(stacks, stack_name)
        self.assertTrue(first_stack.current)

        # Remove both stacks
        ws.remove_stack(stack_name)
        ws.remove_stack(second_stack_name)

        # Check that they are both gone
        stacks = ws.list_stacks()
        first_stack = get_stack(stacks, stack_name)
        second_stack = get_stack(stacks, second_stack_name)
        self.assertIsNone(first_stack)
        self.assertIsNone(second_stack)
