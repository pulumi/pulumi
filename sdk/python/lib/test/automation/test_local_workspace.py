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

import json
import os
import unittest
from semver import VersionInfo
from typing import List, Optional

import pytest

from pulumi import Config, export
from pulumi.automation import (
    create_stack,
    create_or_select_stack,
    CommandError,
    ConfigMap,
    ConfigValue,
    EngineEvent,
    InvalidVersionError,
    LocalWorkspace,
    LocalWorkspaceOptions,
    OpType,
    PluginInfo,
    ProjectSettings,
    StackSummary,
    Stack,
    StackSettings,
    StackAlreadyExistsError,
    fully_qualified_stack_name,
)
from pulumi.automation._local_workspace import _parse_and_validate_pulumi_version

from .test_utils import get_test_org, get_test_suffix, stack_namer

extensions = ["json", "yaml", "yml"]

MAJOR = "Major version mismatch."
MINIMAL = "Minimum version requirement failed."
PARSE = "Could not parse the Pulumi CLI"
version_tests = [
    # current_version, expected_error regex, opt_out
    ("100.0.0", MAJOR, False),
    ("1.0.0", MINIMAL, False),
    ("2.22.0", None, False),
    ("2.1.0", MINIMAL, False),
    ("2.21.2", None, False),
    ("2.21.1", None, False),
    ("2.21.0", MINIMAL, False),
    # Note that prerelease < release so this case will error
    ("2.21.1-alpha.1234", MINIMAL, False),
    # Test opting out of version check
    ("2.20.0", None, True),
    ("2.22.0", None, True),
    # Test invalid version
    ("invalid", PARSE, False),
    ("invalid", None, True),
]
test_min_version = VersionInfo.parse("2.21.1")

def get_test_path(*paths):
    return os.path.join(os.path.dirname(os.path.abspath(__file__)), *paths)


def normalize_config_key(key: str, project_name: str):
    parts = key.split(":")
    if len(parts) < 2:
        return f"{project_name}:{key}"


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


@pytest.mark.skipif("PULUMI_ACCESS_TOKEN" not in os.environ, reason="PULUMI_ACCESS_TOKEN not set")
class TestLocalWorkspace(unittest.TestCase):
    def test_project_settings(self):
        for ext in extensions:
            ws = LocalWorkspace(work_dir=get_test_path("data", ext))
            settings = ws.project_settings()
            self.assertEqual(settings.name, "testproj")
            self.assertEqual(settings.runtime, "go")
            self.assertEqual(settings.description, "A minimal Go Pulumi program")

    def test_stack_settings(self):
        for ext in extensions:
            ws = LocalWorkspace(work_dir=get_test_path("data", ext))
            settings = ws.stack_settings("dev")
            self.assertEqual(settings.secrets_provider, "abc")
            self.assertEqual(settings.encryption_salt, "blahblah")
            self.assertEqual(settings.encrypted_key, "thisiskey")
            self.assertEqual(settings.config["plain"], "plain")
            self.assertEqual(settings.config["secure"].secure, "secret")

        settings_with_no_config = StackSettings(secrets_provider="blah",
                                                encrypted_key="thisiskey",
                                                encryption_salt="salty")
        self.assertEqual(settings_with_no_config._serialize(), {
            "secretsprovider": "blah",
            "encryptedkey": "thisiskey",
            "encryptionsalt": "salty"
        })

        config = {
            "cool": "sup",
            "foo": {"secure": "thisisasecret"},
        }
        settings_with_only_config = StackSettings(config=config)
        self.assertEqual(settings_with_only_config._serialize(), {
            "config": config
        })

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
        self.assertIsNotNone(result.url)

    def test_stack_init(self):
        project_name = "python_test"
        project_settings = ProjectSettings(name=project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)

        Stack.create(stack_name, ws)
        # Trying to create the stack again throws an error
        self.assertRaises(StackAlreadyExistsError, Stack.create, stack_name, ws)
        # Stack.select succeeds
        self.assertEqual(Stack.select(stack_name, ws).name, stack_name)
        # Stack.create_or_select succeeds
        self.assertEqual(Stack.create_or_select(stack_name, ws).name, stack_name)
        ws.remove_stack(stack_name)

    def test_config_env_functions(self):
        if get_test_org() != "moolumi":
            self.skipTest("Skipping test because the required environments are in the moolumi org.")
        project_name = "python_env_test"
        project_settings = ProjectSettings(name=project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
        stack = Stack.create(stack_name, ws)

        # Ensure an env that doesn't exist errors
        self.assertRaises(CommandError, stack.add_environments, "non-existent-env")

        # Ideally here we would be able to check that the envs were added/removed, but the CLI doesn't
        # currently support listing envs from a stack configuration. We can at least check that the
        # commands don't error.

        # Ensure envs that do exist can be added
        stack.add_environments("automation-api-test-env", "automation-api-test-env-2")

        # Check that we can access config from each env.
        config = stack.get_all_config()
        self.assertEqual(config[f"{project_name}:new_key"].value, "test_value")
        self.assertEqual(config[f"{project_name}:also"].value, "business")

        # Ensure envs can be removed
        stack.remove_environment("automation-api-test-env-2")

        # Check that we can still access config from the remaining env,
        # and that the config from the removed env is no longer present.
        self.assertEqual(stack.get_config("new_key").value, "test_value")
        self.assertRaises(CommandError, stack.get_config, "also")

        stack.remove_environment("automation-api-test-env")
        self.assertRaises(CommandError, stack.get_config, "new_key")

        ws.remove_stack(stack_name)

    def test_config_functions(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
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

    def test_config_functions_path(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
        stack = Stack.create(stack_name, ws)

        # test backward compatibility
        stack.set_config("key1", ConfigValue(value="value1"))
        # test new flag without subPath
        stack.set_config("key2", ConfigValue(value="value2"), path=False)
        # test new flag with subPath
        stack.set_config("key3.subKey1", ConfigValue(value="value3"), path=True)
        # test secret
        stack.set_config("key4", ConfigValue(value="value4", secret=True))
        # test subPath and key as secret
        stack.set_config("key5.subKey1", ConfigValue(value="value5", secret=True), path=True)
        # test string with dots
        stack.set_config("key6.subKey1", ConfigValue(value="value6", secret=True))
        # test string with dots
        stack.set_config("key7.subKey1", ConfigValue(value="value7", secret=True), path=False)
        # test subPath
        stack.set_config("key7.subKey2", ConfigValue(value="value8"), path=True)
        # test subPath
        stack.set_config("key7.subKey3", ConfigValue(value="value9"), path=True)

        # test backward compatibility
        cv1 = stack.get_config("key1")
        self.assertEqual(cv1.value, "value1")
        self.assertFalse(cv1.secret)

        # test new flag without subPath
        cv2 = stack.get_config("key2", path=False)
        self.assertEqual(cv2.value, "value2")
        self.assertFalse(cv2.secret)

        # test new flag with subPath
        cv3 = stack.get_config("key3.subKey1", path=True)
        self.assertEqual(cv3.value, "value3")
        self.assertFalse(cv3.secret)

        # test secret
        cv4 = stack.get_config("key4")
        self.assertEqual(cv4.value, "value4")
        self.assertTrue(cv4.secret)

        # test subPath and key as secret
        cv5 = stack.get_config("key5.subKey1", path=True)
        self.assertEqual(cv5.value, "value5")
        self.assertTrue(cv5.secret)

        # test string with dots
        cv6 = stack.get_config("key6.subKey1")
        self.assertEqual(cv6.value, "value6")
        self.assertTrue(cv6.secret)

        # test string with dots
        cv7 = stack.get_config("key7.subKey1", path=False)
        self.assertEqual(cv7.value, "value7")
        self.assertTrue(cv7.secret)

        # test string with dots
        cv8 = stack.get_config("key7.subKey2", path=True)
        self.assertEqual(cv8.value, "value8")
        self.assertFalse(cv8.secret)

        # test string with dots
        cv9 = stack.get_config("key7.subKey3", path=True)
        self.assertEqual(cv9.value, "value9")
        self.assertFalse(cv9.secret)

        stack.remove_config("key1")
        stack.remove_config("key2", path=False)
        stack.remove_config("key3", path=False)
        stack.remove_config("key4", path=False)
        stack.remove_config("key5", path=False)
        stack.remove_config("key6.subKey1", path=False)
        stack.remove_config("key7.subKey1", path=False)

        cfg = stack.get_all_config()
        self.assertEqual(cfg["python_test:key7"].value, '{"subKey2":"value8","subKey3":"value9"}')

        ws.remove_stack(stack_name)

    def test_config_all_functions_path(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
        stack = Stack.create(stack_name, ws)

        stack.set_all_config({
            "key1": ConfigValue(value="value1", secret=False),
            "key2": ConfigValue(value="value2", secret=True),
            "key3.subKey1": ConfigValue(value="value3", secret=False),
            "key3.subKey2": ConfigValue(value="value4", secret=False),
            "key3.subKey3": ConfigValue(value="value5", secret=False),
            "key4.subKey1": ConfigValue(value="value6", secret=True),
        }, path=True)


        # test the SetAllConfigWithOptions configured the first item
        cv1 = stack.get_config("key1")
        self.assertEqual(cv1.value, "value1")
        self.assertFalse(cv1.secret)

        # test the SetAllConfigWithOptions configured the second item
        cv2 = stack.get_config("key2", path=False)
        self.assertEqual(cv2.value, "value2")
        self.assertTrue(cv2.secret)

        # test the SetAllConfigWithOptions configured the third item
        cv3 = stack.get_config("key3.subKey1", path=True)
        self.assertEqual(cv3.value, "value3")
        self.assertFalse(cv3.secret)

        # test the SetAllConfigWithOptions configured the third item
        cv4 = stack.get_config("key3.subKey2", path=True)
        self.assertEqual(cv4.value, "value4")
        self.assertFalse(cv4.secret)

        # test the SetAllConfigWithOptions configured the fourth item
        cv5 = stack.get_config("key4.subKey1", path=True)
        self.assertEqual(cv5.value, "value6")
        self.assertTrue(cv5.secret)

        stack.remove_all_config(["key1", "key2", "key3.subKey1", "key3.subKey2", "key4"], path=True)

        cfg = stack.get_all_config()
        self.assertEqual(cfg["python_test:key3"].value, '{"subKey3":"value5"}')

        ws.remove_stack(stack_name)

    def test_bulk_config_ops(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
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

    def test_config_flag_like(self):
        project_name = "python_test"
        project_settings = ProjectSettings(project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
        stack = Stack.create(stack_name, ws)
        stack.set_config("key", ConfigValue(value="-value"))
        stack.set_config("secret-key", ConfigValue(value="-value", secret=True))
        all_config = stack.get_all_config()
        self.assertFalse(all_config["python_test:key"].secret)
        self.assertEqual(all_config["python_test:key"].value, "-value")
        self.assertTrue(all_config["python_test:secret-key"].secret)
        self.assertEqual(all_config["python_test:secret-key"].value, "-value")
        ws.remove_stack(stack_name)

    # This test requires the existence of a Pulumi.dev.yaml file because we are reading the nested
    # config from the file. This means we can't remove the stack at the end of the test.
    # We should also not include secrets in this config, because the secret encryption is only valid within
    # the context of a stack and org, and running this test in different orgs will fail if there are secrets.
    def test_nested_config(self):
        project_name = "nested_config"
        stack_name = fully_qualified_stack_name(get_test_org(), project_name, "dev")
        project_dir = get_test_path("data", project_name)
        stack = create_or_select_stack(stack_name, work_dir=project_dir)

        all_config = stack.get_all_config()
        outer_val = all_config["nested_config:outer"]
        self.assertFalse(outer_val.secret)
        self.assertEqual(outer_val.value, "{\"inner\":\"my_value\",\"other\":\"something_else\"}")

        list_val = all_config["nested_config:myList"]
        self.assertFalse(list_val.secret)
        self.assertEqual(list_val.value, "[\"one\",\"two\",\"three\"]")

        outer = stack.get_config("outer")
        self.assertFalse(outer.secret)
        self.assertEqual(outer_val.value, "{\"inner\":\"my_value\",\"other\":\"something_else\"}")

        arr = stack.get_config("myList")
        self.assertFalse(arr.secret)
        self.assertEqual(arr.value, "[\"one\",\"two\",\"three\"]")

    def test_tag_methods(self):
        project_name = "python_test"
        runtime = "python"
        project_settings = ProjectSettings(name=project_name, runtime=runtime)
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
        _ = Stack.create(stack_name, ws)

        # Lists tag values
        result = ws.list_tags(stack_name)
        self.assertEqual(result["pulumi:project"], project_name)
        self.assertEqual(result["pulumi:runtime"], runtime)

        # Sets tag values
        ws.set_tag(stack_name, "foo", "bar")
        result = ws.list_tags(stack_name)
        self.assertEqual(result["foo"], "bar")

        # Removes tag values
        ws.remove_tag(stack_name, "foo")
        result = ws.list_tags(stack_name)
        self.assertTrue("foo" not in result)

        # Gets a single tag value
        result = ws.get_tag(stack_name, "pulumi:project")
        self.assertEqual(result, project_name)

        ws.remove_stack(stack_name)

    def test_stack_status_methods(self):
        project_name = "python_test"
        project_settings = ProjectSettings(name=project_name, runtime="python")
        ws = LocalWorkspace(project_settings=project_settings)
        stack_name = stack_namer(project_name)
        stack = Stack.create(stack_name, ws)

        history = stack.history()
        self.assertEqual(len(history), 0)
        info = stack.info()
        self.assertIsNone(info)

        ws.remove_stack(stack_name)

    def test_stack_lifecycle_local_program(self):
        project_name = "testproj"
        stack_name = stack_namer(project_name)
        work_dir = get_test_path("data", project_name)
        stack = create_stack(stack_name, work_dir=work_dir)
        self.assertIsNone(print(stack))

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
        preview_result = stack.preview()
        self.assertEqual(preview_result.change_summary.get(OpType.SAME), 1)

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
        project_name = "inline_python"
        stack_name = stack_namer(project_name)
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
            preview_result = stack.preview()
            self.assertEqual(preview_result.change_summary.get(OpType.SAME), 1)

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

    def test_supports_stack_outputs(self):
        project_name = "inline_python"
        stack_name = stack_namer(project_name)
        stack = create_stack(stack_name, program=pulumi_program, project_name=project_name)

        stack_config: ConfigMap = {
            "bar": ConfigValue(value="abc"),
            "buzz": ConfigValue(value="secret", secret=True)
        }

        def assert_outputs(outputs):
            self.assertEqual(len(outputs), 3)
            self.assertEqual(outputs["exp_static"].value, "foo")
            self.assertFalse(outputs["exp_static"].secret)
            self.assertEqual(outputs["exp_cfg"].value, "abc")
            self.assertFalse(outputs["exp_cfg"].secret)
            self.assertEqual(outputs["exp_secret"].value, "secret")
            self.assertTrue(outputs["exp_secret"].secret)

        try:
            stack.set_all_config(stack_config)

            initial_outputs = stack.outputs()
            self.assertEqual(len(initial_outputs), 0)

            # pulumi up
            up_res = stack.up()
            self.assertEqual(up_res.summary.kind, "update")
            self.assertEqual(up_res.summary.result, "succeeded")
            assert_outputs(up_res.outputs)

            outputs_after_up = stack.outputs()
            assert_outputs(outputs_after_up)

            # pulumi destroy
            destroy_res = stack.destroy()
            self.assertEqual(destroy_res.summary.kind, "destroy")
            self.assertEqual(destroy_res.summary.result, "succeeded")

            outputs_after_destroy = stack.outputs()
            self.assertEqual(len(outputs_after_destroy), 0)
        finally:
            stack.workspace.remove_stack(stack_name)

    def test_pulumi_version(self):
        ws = LocalWorkspace()
        self.assertIsNotNone(ws.pulumi_version)
        self.assertRegex(ws.pulumi_version, r"(\d+\.)(\d+\.)(\d+)(-.*)?")

    def test_validate_pulumi_version(self):
        for current_version, expected_error, opt_out in version_tests:
            with self.subTest():
                if expected_error:
                    with self.assertRaisesRegex(
                            InvalidVersionError,
                            expected_error,
                            msg=f"min_version:{test_min_version}, current_version:{current_version}"
                    ):
                        _parse_and_validate_pulumi_version(test_min_version, current_version, opt_out)
                else:
                    _parse_and_validate_pulumi_version(test_min_version, current_version, opt_out)

    def test_project_settings_respected(self):
        project_name = "correct_project"
        stack_name = stack_namer(project_name)
        stack = create_stack(stack_name,
                             program=pulumi_program,
                             project_name=project_name,
                             opts=LocalWorkspaceOptions(work_dir=get_test_path("data", project_name)))
        project_settings = stack.workspace.project_settings()
        self.assertEqual(project_settings.description, "This is a description")
        stack.workspace.remove_stack(stack_name)

    def test_project_settings_populates_main(self):
        main_cases = [
            ('none', None, os.getcwd()),
            ('blank', '', ''),
            ('string', 'foo', 'foo'),
        ]

        for case_name, initial_main, expected_main in main_cases:
            project_name = f"project_populates_main_with_{case_name}"
            stack_name = stack_namer(project_name)
            project_settings = ProjectSettings(name=project_name, runtime="python", main=initial_main)
            stack = create_stack(stack_name,
                                program=pulumi_program,
                                project_name=project_name,
                                opts=LocalWorkspaceOptions(project_settings=project_settings))
            project_settings = stack.workspace.project_settings()
            self.assertEqual(expected_main, project_settings.main)
            stack.workspace.remove_stack(stack_name)


    def test_structured_events(self):
        project_name = "structured_events"
        stack_name = stack_namer(project_name)
        stack = create_stack(stack_name, program=pulumi_program, project_name=project_name)

        stack_config: ConfigMap = {
            "bar": ConfigValue(value="abc"),
            "buzz": ConfigValue(value="secret", secret=True)
        }

        try:
            stack.set_all_config(stack_config)

            # can't mutate a bool from the callback, so use a single-item list
            seen_summary_event = [False]

            def find_summary_event(event: EngineEvent):
                if event.summary_event:
                    seen_summary_event[0] = True

            # pulumi up
            up_res = stack.up(on_event=find_summary_event)
            self.assertEqual(seen_summary_event[0], True, "No SummaryEvent for `up`")
            self.assertEqual(up_res.summary.kind, "update")
            self.assertEqual(up_res.summary.result, "succeeded")

            # pulumi preview
            seen_summary_event[0] = False
            pre_res = stack.preview(on_event=find_summary_event)
            self.assertEqual(seen_summary_event[0], True, "No SummaryEvent for `preview`")
            self.assertEqual(pre_res.change_summary.get(OpType.SAME), 1)

            # pulumi refresh
            seen_summary_event[0] = False
            refresh_res = stack.refresh(on_event=find_summary_event)
            self.assertEqual(seen_summary_event[0], True, "No SummaryEvent for `refresh`")
            self.assertEqual(refresh_res.summary.kind, "refresh")
            self.assertEqual(refresh_res.summary.result, "succeeded")

            # pulumi destroy
            seen_summary_event[0] = False
            destroy_res = stack.destroy(on_event=find_summary_event)
            self.assertEqual(seen_summary_event[0], True, "No SummaryEvent for `destroy`")
            self.assertEqual(destroy_res.summary.kind, "destroy")
            self.assertEqual(destroy_res.summary.result, "succeeded")
        finally:
            stack.workspace.remove_stack(stack_name)

    # TODO[pulumi/pulumi#7127]: Re-enabled the warning.
    @unittest.skip("Temporarily skipping test until we've re-enabled the warning - pulumi/pulumi#7127")
    def test_secret_config_warnings(self):
        def program():
            config = Config()

            config.get("plainstr1")
            config.require("plainstr2")
            config.get_secret("plainstr3")
            config.require_secret("plainstr4")

            config.get_bool("plainbool1")
            config.require_bool("plainbool2")
            config.get_secret_bool("plainbool3")
            config.require_secret_bool("plainbool4")

            config.get_int("plainint1")
            config.require_int("plainint2")
            config.get_secret_int("plainint3")
            config.require_secret_int("plainint4")

            config.get_float("plainfloat1")
            config.require_float("plainfloat2")
            config.get_secret_float("plainfloat3")
            config.require_secret_float("plainfloat4")

            config.get_object("plainobj1")
            config.require_object("plainobj2")
            config.get_secret_object("plainobj3")
            config.require_secret_object("plainobj4")

            config.get("str1")
            config.require("str2")
            config.get_secret("str3")
            config.require_secret("str4")

            config.get_bool("bool1")
            config.require_bool("bool2")
            config.get_secret_bool("bool3")
            config.require_secret_bool("bool4")

            config.get_int("int1")
            config.require_int("int2")
            config.get_secret_int("int3")
            config.require_secret_int("int4")

            config.get_float("float1")
            config.require_float("float2")
            config.get_secret_float("float3")
            config.require_secret_float("float4")

            config.get_object("obj1")
            config.require_object("obj2")
            config.get_secret_object("obj3")
            config.require_secret_object("obj4")

        project_name = "inline_python"
        stack_name = stack_namer(project_name)
        stack = create_stack(stack_name, program=program, project_name=project_name)

        stack_config: ConfigMap = {
            "plainstr1": ConfigValue(value="1"),
            "plainstr2": ConfigValue(value="2"),
            "plainstr3": ConfigValue(value="3"),
            "plainstr4": ConfigValue(value="4"),
            "plainbool1": ConfigValue(value="true"),
            "plainbool2": ConfigValue(value="true"),
            "plainbool3": ConfigValue(value="true"),
            "plainbool4": ConfigValue(value="true"),
            "plainint1": ConfigValue(value="1"),
            "plainint2": ConfigValue(value="2"),
            "plainint3": ConfigValue(value="3"),
            "plainint4": ConfigValue(value="4"),
            "plainfloat1": ConfigValue(value="1.1"),
            "plainfloat2": ConfigValue(value="2.2"),
            "plainfloat3": ConfigValue(value="3.3"),
            "plainfloat4": ConfigValue(value="4.3"),
            "plainobj1": ConfigValue(value="{}"),
            "plainobj2": ConfigValue(value="{}"),
            "plainobj3": ConfigValue(value="{}"),
            "plainobj4": ConfigValue(value="{}"),
            "str1": ConfigValue(value="1", secret=True),
            "str2": ConfigValue(value="2", secret=True),
            "str3": ConfigValue(value="3", secret=True),
            "str4": ConfigValue(value="4", secret=True),
            "bool1": ConfigValue(value="true", secret=True),
            "bool2": ConfigValue(value="true", secret=True),
            "bool3": ConfigValue(value="true", secret=True),
            "bool4": ConfigValue(value="true", secret=True),
            "int1": ConfigValue(value="1", secret=True),
            "int2": ConfigValue(value="2", secret=True),
            "int3": ConfigValue(value="3", secret=True),
            "int4": ConfigValue(value="4", secret=True),
            "float1": ConfigValue(value="1.1", secret=True),
            "float2": ConfigValue(value="2.2", secret=True),
            "float3": ConfigValue(value="3.3", secret=True),
            "float4": ConfigValue(value="4.4", secret=True),
            "obj1": ConfigValue(value="{}", secret=True),
            "obj2": ConfigValue(value="{}", secret=True),
            "obj3": ConfigValue(value="{}", secret=True),
            "obj4": ConfigValue(value="{}", secret=True),
        }

        try:
            stack.set_all_config(stack_config)

            events: List[str] = []

            def find_diagnostic_events(event: EngineEvent):
                if event.diagnostic_event and event.diagnostic_event.severity == "warning":
                    events.append(event.diagnostic_event.message)

            expected_warnings = [
                "Configuration 'inline_python:str1' value is a secret; use `get_secret` instead of `get`",
                "Configuration 'inline_python:str2' value is a secret; use `require_secret` instead of `require`",
                "Configuration 'inline_python:bool1' value is a secret; use `get_secret_bool` instead of `get_bool`",
                "Configuration 'inline_python:bool2' value is a secret; use `require_secret_bool` instead of `require_bool`",
                "Configuration 'inline_python:int1' value is a secret; use `get_secret_int` instead of `get_int`",
                "Configuration 'inline_python:int2' value is a secret; use `require_secret_int` instead of `require_int`",
                "Configuration 'inline_python:float1' value is a secret; use `get_secret_float` instead of `get_float`",
                "Configuration 'inline_python:float2' value is a secret; use `require_secret_float` instead of `require_float`",
                "Configuration 'inline_python:obj1' value is a secret; use `get_secret_object` instead of `get_object`",
                "Configuration 'inline_python:obj2' value is a secret; use `require_secret_object` instead of `require_object`",
            ]

            # These keys should not be in any warning messages.
            unexpected_warnings = [
                "plainstr1",
                "plainstr2",
                "plainstr3",
                "plainstr4",
                "plainbool1",
                "plainbool2",
                "plainbool3",
                "plainbool4",
                "plainint1",
                "plainint2",
                "plainint3",
                "plainint4",
                "plainfloat1",
                "plainfloat2",
                "plainfloat3",
                "plainfloat4",
                "plainobj1",
                "plainobj2",
                "plainobj3",
                "plainobj4",
                "str3",
                "str4",
                "bool3",
                "bool4",
                "int3",
                "int4",
                "float3",
                "float4",
                "obj3",
                "obj4",
            ]

            def validate(warnings: List[str]):
                for expected in expected_warnings:
                    found = False
                    for warning in warnings:
                        if expected in warning:
                            found = True
                            break
                    self.assertTrue(found, "expected warning not found")
                for unexpected in unexpected_warnings:
                    for warning in warnings:
                        self.assertFalse(unexpected in warning, f"Unexpected ${unexpected}' found in warning")

            # pulumi preview
            stack.preview(on_event=find_diagnostic_events)
            validate(events)

            # pulumi up
            events = []
            stack.up(on_event=find_diagnostic_events)
            validate(events)
        finally:
            stack.workspace.remove_stack(stack_name)


def pulumi_program():
    config = Config()
    export("exp_static", "foo")
    export("exp_cfg", config.get("bar"))
    export("exp_secret", config.get_secret("buzz"))


@pytest.mark.parametrize("key,default", [("string", None), ("bar", "baz"), ("doesnt-exist", None)])
def test_config_get_with_defaults(key, default, mock_config, config_settings):
    assert mock_config.get(key, default) == config_settings.get(f"test-config:{key}", default)


def test_config_get_int(mock_config, config_settings):
    assert mock_config.get_int("int") == int(config_settings.get("test-config:int"))


def test_config_get_bool(mock_config):
    assert mock_config.get_bool("bool") is False


def test_config_get_object(mock_config, config_settings):
    assert mock_config.get_object("object") == json.loads(config_settings.get("test-config:object"))


def test_config_get_float(mock_config, config_settings):
    assert mock_config.get_float("float") == float(config_settings.get("test-config:float"))
