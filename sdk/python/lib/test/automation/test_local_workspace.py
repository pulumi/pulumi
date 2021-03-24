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
import unittest
from random import random
from semver import VersionInfo
from typing import List, Optional

from pulumi import Config, export
from pulumi.automation import (
    create_stack,
    create_or_select_stack,
    CommandError,
    ConfigMap,
    ConfigValue,
    InvalidVersionError,
    LocalWorkspace,
    LocalWorkspaceOptions,
    PluginInfo,
    ProjectSettings,
    StackSummary,
    Stack,
    StackAlreadyExistsError,
    fully_qualified_stack_name,
)
from pulumi.automation._local_workspace import _validate_pulumi_version

extensions = ["json", "yaml", "yml"]

version_tests = [
    ("100.0.0", True),
    ("1.0.0", True),
    ("2.22.0", False),
    ("2.1.0", True),
    ("2.21.2", False),
    ("2.21.1", False),
    ("2.21.0", True),
    # Note that prerelease < release so this case will error
    ("2.21.1-alpha.1234", True)
]
test_min_version = VersionInfo.parse("2.21.1")


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

        Stack.create(stack_name, ws)
        # Trying to create the stack again throws an error
        self.assertRaises(StackAlreadyExistsError, Stack.create, stack_name, ws)
        # Stack.select succeeds
        self.assertEqual(Stack.select(stack_name, ws).name, stack_name)
        # Stack.create_or_select succeeds
        self.assertEqual(Stack.create_or_select(stack_name, ws).name, stack_name)
        ws.remove_stack(stack_name)

    def test_config_functions(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer()
        stack = Stack.create(stack_name, ws)

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

    def test_bulk_config_ops(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer()
        stack = Stack.create(stack_name, ws)

        config: ConfigMap = {
            "one": ConfigValue(value="one"),
            "two": ConfigValue(value="two"),
            "three": ConfigValue(value="three", secret=True),
            "four": ConfigValue(value="four", secret=True),
            "five": ConfigValue(value="five"),
            "six": ConfigValue(value="six"),
            "seven": ConfigValue(value="seven", secret=True),
            "eight": ConfigValue(value="eight", secret=True),
            "nine": ConfigValue(value="nine"),
            "ten": ConfigValue(value="ten"),
        }
        stack.set_all_config(config)
        stack.remove_all_config([key for key in config])

        ws.remove_stack(stack_name)

    def test_nested_config(self):
        stack_name = fully_qualified_stack_name("pulumi-test", "nested_config", "dev")
        project_dir = test_path("data", "nested_config")
        stack = create_or_select_stack(stack_name, work_dir=project_dir)

        all_config = stack.get_all_config()
        outer_val = all_config["nested_config:outer"]
        self.assertTrue(outer_val.secret)
        self.assertEqual(outer_val.value, "{\"inner\":\"my_secret\",\"other\":\"something_else\"}")

        list_val = all_config["nested_config:myList"]
        self.assertFalse(list_val.secret)
        self.assertEqual(list_val.value, "[\"one\",\"two\",\"three\"]")

        outer = stack.get_config("outer")
        self.assertTrue(outer.secret)
        self.assertEqual(outer_val.value, "{\"inner\":\"my_secret\",\"other\":\"something_else\"}")

        arr = stack.get_config("myList")
        self.assertFalse(arr.secret)
        self.assertEqual(arr.value, "[\"one\",\"two\",\"three\"]")

    def test_stack_status_methods(self):
        project_settings = ProjectSettings(name="python_test", runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer()
        stack = Stack.create(stack_name, ws)

        history = stack.history()
        self.assertEqual(len(history), 0)
        info = stack.info()
        self.assertIsNone(info)

        ws.remove_stack(stack_name)

    def test_stack_lifecycle_local_program(self):
        stack_name = stack_namer()
        work_dir = test_path("data", "testproj")
        stack = create_stack(stack_name, work_dir=work_dir)

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

        stack.workspace.remove_stack(stack_name)

    def test_stack_lifecycle_inline_program(self):
        stack_name = stack_namer()
        project_name = "inline_python"
        stack = create_stack(stack_name, program=pulumi_program, project_name=project_name)

        stack_config: ConfigMap = {
            "bar": ConfigValue(value="abc"),
            "buzz": ConfigValue(value="secret", secret=True)
        }

        try:
            stack.set_all_config(stack_config)

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
        finally:
            stack.workspace.remove_stack(stack_name)

    def test_pulumi_version(self):
        ws = LocalWorkspace()
        self.assertIsNotNone(ws.pulumi_version)
        self.assertRegex(ws.pulumi_version, r"(\d+\.)(\d+\.)(\d+)(-.*)?")

    def test_validate_pulumi_version(self):
        for current_version, expect_error in version_tests:
            with self.subTest():
                current_version = VersionInfo.parse(current_version)
                if expect_error:
                    error_regex = "Major version mismatch." \
                        if test_min_version.major < current_version.major \
                        else "Minimum version requirement failed."
                    with self.assertRaisesRegex(
                            InvalidVersionError,
                            error_regex,
                            msg=f"min_version:{test_min_version}, current_version:{current_version}"
                    ):
                        _validate_pulumi_version(test_min_version, current_version)
                else:
                    self.assertIsNone(_validate_pulumi_version(test_min_version, current_version))

    def test_project_settings_respected(self):
        stack_name = stack_namer()
        project_name = "project_was_overwritten"
        stack = create_stack(stack_name,
                             program=pulumi_program,
                             project_name=project_name,
                             opts=LocalWorkspaceOptions(work_dir=test_path("data", "correct_project")))
        project_settings = stack.workspace.project_settings()
        self.assertEqual(project_settings.name, "correct_project")
        self.assertEqual(project_settings.description, "This is a description")
        stack.workspace.remove_stack(stack_name)


def pulumi_program():
    config = Config()
    export("exp_static", "foo")
    export("exp_cfg", config.get("bar"))
    export("exp_secret", config.get_secret("buzz"))
