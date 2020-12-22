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
from pulumi.x.automation import LocalWorkspace, PluginInfo
from typing import List

extensions = ["json", "yaml", "yml"]


def found_plugin(plugin_list: List[PluginInfo], name: str, version: str) -> bool:
    for plugin in plugin_list:
        if plugin.name == name and plugin.version == version:
            return True
    return False


class TestLocalWorkspace(unittest.TestCase):
    def test_project_settings(self):
        for ext in extensions:
            ws = LocalWorkspace(work_dir=os.path.join(os.getcwd(), "data", ext))
            settings = ws.project_settings()
            self.assertEqual(settings.name, "testproj")
            self.assertEqual(settings.runtime, "go")
            self.assertEqual(settings.description, "A minimal Go Pulumi program")

    def test_stack_settings(self):
        for ext in extensions:
            ws = LocalWorkspace(work_dir=os.path.join(os.getcwd(), "data", ext))
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
