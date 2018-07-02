# Copyright 2016-2018, Pulumi Corporation.
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
from pulumi.runtime import Unknown



class UnknownTests(unittest.TestCase):
    """
    Tests that Unknown works well enough to get us through previews.
    """
    def test_nonzero_bool(self):
        unknown = Unknown()
        self.assertFalse(not unknown)
        if not unknown:
            self.fail("if boolean conversion failed")
        self.assertTrue(bool(unknown))

    def test_attr(self):
        class MockPreviewObj(object):
            def __init__(self):
                self.id = Unknown()
                self.output = Unknown()

        preview_obj = MockPreviewObj()
        self.assertIsInstance(preview_obj.id, Unknown)
        self.assertIsInstance(preview_obj.output, Unknown)

    def test_str(self):
        unknown = Unknown()
        self.assertEqual(str(unknown), "<computed>")

    def test_dict_list(self):
        unknown = Unknown()
        self.assertIsInstance(unknown["foo"], Unknown)
        self.assertIsInstance(unknown[0], Unknown)

    def test_iter_dir(self):
        unknown = Unknown()
        for _ in unknown:
            self.fail("Unknown iterator should be empty")

        for _ in dir(unknown):
            self.fail("Dir should return empty")
