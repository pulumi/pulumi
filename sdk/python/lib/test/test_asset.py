# Copyright 2026, Pulumi Corporation.
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

import pulumi
from pulumi import StringAsset, string_asset_output
from pulumi.output import Output
from pulumi.runtime import rpc, settings


def pulumi_test(coro):
    wrapped = pulumi.runtime.test(coro)

    def wrapper(*args, **kwargs):
        settings.configure(settings.Settings("project", "stack"))
        rpc._RESOURCE_PACKAGES.clear()
        rpc._RESOURCE_MODULES.clear()
        wrapped(*args, **kwargs)

    return wrapper


class StringAssetOutputTests(unittest.IsolatedAsyncioTestCase):
    @pulumi_test
    async def test_from_plain_string(self):
        out = string_asset_output("hello")
        val = await out.future()
        self.assertIsInstance(val, StringAsset)
        self.assertEqual(val.text, "hello")

    @pulumi_test
    async def test_from_output_string(self):
        str_out = Output.from_input("world")
        out = string_asset_output(str_out)
        val = await out.future()
        self.assertIsInstance(val, StringAsset)
        self.assertEqual(val.text, "world")

    @pulumi_test
    async def test_result_is_output(self):
        out = string_asset_output("test")
        self.assertIsInstance(out, Output)

    @pulumi_test
    async def test_secret_output_preserves_secret(self):
        str_out = Output.secret("my-secret")
        out = string_asset_output(str_out)
        is_secret = await out.is_secret()
        self.assertTrue(is_secret)
        val = await out.future()
        self.assertIsInstance(val, StringAsset)
        self.assertEqual(val.text, "my-secret")
