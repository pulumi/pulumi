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
from pulumi.automation._local_workspace import _validate_pulumi_version

extensions = ["json", "yaml", "yml"]

version_tests = [
    ("100.0.0", True, False),
    ("1.0.0", True, False),
    ("2.22.0", False, False),
    ("2.1.0", True, False),
    ("2.21.2", False, False),
    ("2.21.1", False, False),
    ("2.21.0", True, False),
    # Note that prerelease < release so this case will error
    ("2.21.1-alpha.1234", True, False),
    # Test opting out of version check
    ("2.20.0", False, True),
    ("2.22.0", False, True)
]
test_min_version = VersionInfo.parse("2.21.1")


def test_path(*paths):
    return os.path.join(os.path.dirname(os.path.abspath(__file__)), *paths)


def get_test_org():
    test_org = "pulumi-test"
    env_var = os.getenv("PULUMI_TEST_ORG")
    if env_var is not None:
        test_org = env_var
    return test_org


def stack_namer(project_name):
    return fully_qualified_stack_name(get_test_org(), project_name, f"int_test_{get_test_suffix()}")


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

    def test_nested_config(self):
        if get_test_org() != "pulumi-test":
            return
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
        work_dir = test_path("data", project_name)
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
        for current_version, expect_error, opt_out in version_tests:
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
                        _validate_pulumi_version(test_min_version, current_version, opt_out)
                else:
                    self.assertIsNone(_validate_pulumi_version(test_min_version, current_version, opt_out))

    def test_project_settings_respected(self):
        project_name = "correct_project"
        stack_name = stack_namer(project_name)
        stack = create_stack(stack_name,
                             program=pulumi_program,
                             project_name=project_name,
                             opts=LocalWorkspaceOptions(work_dir=test_path("data", project_name)))
        project_settings = stack.workspace.project_settings()
        self.assertEqual(project_settings.description, "This is a description")
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
