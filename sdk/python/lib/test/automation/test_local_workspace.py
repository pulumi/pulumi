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
from typing import List, Optional

from pulumi.x.automation import (
    CommandError,
    ConfigMap,
    ConfigValue,
    LocalWorkspace,
    PluginInfo,
    ProjectSettings,
    StackSummary,
    Stack,
    StackAlreadyExistsError
)


extensions = ["json", "yaml", "yml"]


def test_path(*paths):
    return os.path.join(os.path.dirname(os.path.abspath(__file__)), *paths)


def stack_namer():
    return f"int_test_{get_test_suffix()}"


def normalize_config_key(key: str, project_name: str):
    parts = key.split(":")
    if len(parts) < 2:
        return f"{project_name}:{key}"


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
        stack_1_name = f"python_int_test_first_{get_test_suffix()}"
        stack_2_name = f"python_int_test_second_{get_test_suffix()}"

        # Create a stack
        ws.create_stack(stack_1_name)
        stacks = ws.list_stacks()
        stack_1 = get_stack(stacks, stack_1_name)

        # Check the stack exists
        self.assertIsNotNone(stack_1)
        # Check that it's current
        self.assertTrue(stack_1.current)

        # Create another stack
        ws.create_stack(stack_2_name)
        stacks = ws.list_stacks()
        stack_1 = get_stack(stacks, stack_1_name)
        stack_2 = get_stack(stacks, stack_2_name)

        # Check the second stack exists
        self.assertIsNotNone(stack_2)
        # Check that second stack is current but the first is not
        self.assertFalse(stack_1.current)
        self.assertTrue(stack_2.current)

        # Select the first stack again
        ws.select_stack(stack_1_name)
        stacks = ws.list_stacks()
        stack_1 = get_stack(stacks, stack_1_name)

        # Check the first stack is now current
        self.assertTrue(stack_1.current)

        # Get the current stack info
        current_stack = ws.stack()

        # Check that the name matches stack 1
        self.assertEqual(current_stack.name, stack_1_name)

        # Remove both stacks
        ws.remove_stack(stack_1_name)
        ws.remove_stack(stack_2_name)
        stacks = ws.list_stacks()
        stack_1 = get_stack(stacks, stack_1_name)
        stack_2 = get_stack(stacks, stack_2_name)

        # Check that they were both removed
        self.assertIsNone(stack_1)
        self.assertIsNone(stack_2)

    def test_who_am_i(self):
        ws = LocalWorkspace()
        result = ws.who_am_i()
        self.assertIsNotNone(result.user)

    def test_stack_init(self):
        project_settings = ProjectSettings(name="python_test", runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer()

        Stack(stack_name, ws)
        # Trying to create the stack again throws an error
        self.assertRaises(StackAlreadyExistsError, Stack, stack_name, ws)
        # Setting select_if_exists succeeds
        self.assertEqual(Stack(stack_name, ws, select_if_exists=True).name, stack_name)
        ws.remove_stack(stack_name)

    def test_config_functions(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer()
        stack = Stack(stack_name, ws)

        config: ConfigMap = {
            "plain": ConfigValue(value="abc"),
            "secret": ConfigValue(value="def", secret=True)
        }

        plain_key = normalize_config_key("plain", project_name)
        secret_key = normalize_config_key("secret", project_name)

        self.assertRaises(CommandError, stack.get_config, plain_key)

        values = stack.get_all_config()
        self.assertEqual(len(values), 0)

        stack.set_all_config(config)
        values = stack.get_all_config()
        self.assertEqual(values[plain_key].value, "abc")
        self.assertFalse(values[plain_key].secret)
        self.assertEqual(values[secret_key].value, "def")
        self.assertTrue(values[secret_key].secret)

        stack.remove_config("plain")
        values = stack.get_all_config()
        self.assertEqual(len(values), 1)

        stack.set_config("foo", ConfigValue(value="bar"))
        values = stack.get_all_config()
        self.assertEqual(len(values), 2)

        ws.remove_stack(stack_name)

    def test_stack_status_methods(self):
        project_settings = ProjectSettings(name="python_test", runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer()
        stack = Stack(stack_name, ws)

        history = stack.history()
        self.assertEqual(len(history), 0)
        info = stack.info()
        self.assertIsNone(info)

        ws.remove_stack(stack_name)

    def test_stack_lifecycle_local_program(self):
        stack_name = stack_namer()
        work_dir = test_path("data", "testproj")
        ws = LocalWorkspace(work_dir=work_dir)
        stack = Stack(stack_name, ws)

        config: ConfigMap = {
            "bar": ConfigValue(value="abc"),
            "buzz": ConfigValue(value="secret", secret=True)
        }
        stack.set_all_config(config)

        # pulumi up
        up_res = stack.up()
        self.assertEqual(len(up_res.outputs), 3)
        self.assertEqual(up_res.outputs["exp_static"].value, "foo")
        self.assertFalse(up_res.outputs["exp_static"].secret)
        self.assertEqual(up_res.outputs["exp_cfg"].value, "abc")
        self.assertFalse(up_res.outputs["exp_cfg"].secret)
        self.assertEqual(up_res.outputs["exp_secret"].value, "secret")
        self.assertTrue(up_res.outputs["exp_secret"].secret)
        self.assertEqual(up_res.summary.kind, "update")
        self.assertEqual(up_res.summary.result, "succeeded")

        # pulumi preview
        stack.preview()
        # TODO: update assertions when we have structured output

        # pulumi refresh
        refresh_res = stack.refresh()
        self.assertEqual(refresh_res.summary.kind, "refresh")
        self.assertEqual(refresh_res.summary.result, "succeeded")

        # pulumi destroy
        destroy_res = stack.destroy()
        self.assertEqual(destroy_res.summary.kind, "destroy")
        self.assertEqual(destroy_res.summary.result, "succeeded")
