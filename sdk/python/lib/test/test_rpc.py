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

from pulumi.output import Unknown, UNKNOWN
from pulumi.runtime import rpc


class TestContainsUnknowns(unittest.TestCase):
    """Test suite for the contains_unknowns function."""

    def test_unknown_value(self):
        self.assertTrue(rpc.contains_unknowns(UNKNOWN))
        self.assertTrue(rpc.contains_unknowns(Unknown()))

    def test_known_primitives(self):
        self.assertFalse(rpc.contains_unknowns(None))
        self.assertFalse(rpc.contains_unknowns(True))
        self.assertFalse(rpc.contains_unknowns(False))
        self.assertFalse(rpc.contains_unknowns(42))
        self.assertFalse(rpc.contains_unknowns(3.14))
        self.assertFalse(rpc.contains_unknowns("hello"))
        self.assertFalse(rpc.contains_unknowns(b"bytes"))

    def test_empty_collections(self):
        self.assertFalse(rpc.contains_unknowns([]))
        self.assertFalse(rpc.contains_unknowns({}))
        self.assertFalse(rpc.contains_unknowns(()))

    def test_list(self):
        self.assertFalse(rpc.contains_unknowns([1, 2, 3]))
        self.assertFalse(rpc.contains_unknowns(["a", "b", "c"]))
        self.assertFalse(rpc.contains_unknowns([None, True, False]))
        self.assertFalse(rpc.contains_unknowns([[1, 2], [3, 4]]))
        self.assertFalse(rpc.contains_unknowns([[[1]], [[2]], [[3]]]))

        self.assertTrue(rpc.contains_unknowns([UNKNOWN]))
        self.assertTrue(rpc.contains_unknowns([1, 2, UNKNOWN]))
        self.assertTrue(rpc.contains_unknowns([1, 2, 3, UNKNOWN, 4, 5]))
        self.assertTrue(rpc.contains_unknowns([[UNKNOWN]]))
        self.assertTrue(rpc.contains_unknowns([1, [2, [3, UNKNOWN]]]))
        self.assertTrue(rpc.contains_unknowns([[1, 2], [3, [4, UNKNOWN]]]))

    def test_dict_with_unknown_value(self):
        self.assertTrue(rpc.contains_unknowns({"key": UNKNOWN}))
        self.assertTrue(rpc.contains_unknowns({"a": 1, "b": UNKNOWN}))
        self.assertTrue(rpc.contains_unknowns({"known": "value", "unknown": UNKNOWN}))
        self.assertTrue(rpc.contains_unknowns({"outer": {"inner": UNKNOWN}}))
        self.assertTrue(rpc.contains_unknowns({"a": {"b": {"c": {"d": UNKNOWN}}}}))
        self.assertTrue(
            rpc.contains_unknowns(
                {"known": {"values": [1, 2, 3]}, "unknown": {"value": UNKNOWN}}
            )
        )

        self.assertFalse(rpc.contains_unknowns({"a": 1, "b": 2}))
        self.assertFalse(rpc.contains_unknowns({"name": "test", "value": 42}))
        self.assertFalse(rpc.contains_unknowns({"outer": {"inner": "value"}}))
        self.assertFalse(rpc.contains_unknowns({"a": {"b": {"c": {"d": 42}}}}))

    def test_mixed_nested_structures_with_unknown(self):
        self.assertTrue(rpc.contains_unknowns({"list": [1, 2, {"nested": UNKNOWN}]}))
        self.assertTrue(rpc.contains_unknowns([{"a": 1}, {"b": [2, 3, UNKNOWN]}]))
        self.assertTrue(
            rpc.contains_unknowns(
                {
                    "config": {
                        "servers": [
                            {"host": "localhost", "port": 8080},
                            {"host": "remote", "port": UNKNOWN},
                        ]
                    }
                }
            )
        )
        self.assertFalse(rpc.contains_unknowns({"list": [1, 2, {"nested": "value"}]}))
        self.assertFalse(rpc.contains_unknowns([{"a": 1}, {"b": [2, 3, 4]}]))

    def test_circular_reference_list(self):
        """Test that circular references in lists are handled."""
        lst = [1, 2, 3]
        lst.append(lst)  # Create circular reference
        self.assertFalse(rpc.contains_unknowns(lst))

        lst_with_unknown = [1, 2, UNKNOWN]
        lst_with_unknown.append(lst_with_unknown)
        self.assertTrue(rpc.contains_unknowns(lst_with_unknown))

    def test_circular_reference_dict(self):
        """Test that circular references in dicts are handled."""
        d = {"a": 1, "b": 2}
        d["self"] = d  # Create circular reference
        self.assertFalse(rpc.contains_unknowns(d))

        d_with_unknown = {"a": 1, "b": UNKNOWN}
        d_with_unknown["self"] = d_with_unknown
        self.assertTrue(rpc.contains_unknowns(d_with_unknown))

    def test_circular_reference_mixed(self):
        """Test circular references between dicts and lists."""
        d = {"list": []}
        lst = [d]
        d["list"] = lst  # Create circular reference
        self.assertFalse(rpc.contains_unknowns(d))

        d_unknown = {"list": [], "value": UNKNOWN}
        lst_unknown = [d_unknown]
        d_unknown["list"] = lst_unknown
        self.assertTrue(rpc.contains_unknowns(d_unknown))

    def test_shared_reference(self):
        shared = {"shared": "value"}
        container = {"ref1": shared, "ref2": shared}
        self.assertFalse(rpc.contains_unknowns(container))

        shared_unknown = {"shared": UNKNOWN}
        container_unknown = {"ref1": shared_unknown, "ref2": shared_unknown}
        self.assertTrue(rpc.contains_unknowns(container_unknown))

    def test_deeply_nested_structure(self):
        deep = {"value": 42}
        for i in range(100):
            deep = {"level": deep}
        self.assertFalse(rpc.contains_unknowns(deep))

        deep_unknown = {"value": UNKNOWN}
        for i in range(100):
            deep_unknown = {"level": deep_unknown}
        self.assertTrue(rpc.contains_unknowns(deep_unknown))

    def test_wide_structure(self):
        wide_dict = {f"key_{i}": i for i in range(1000)}
        self.assertFalse(rpc.contains_unknowns(wide_dict))

        wide_dict["key_500"] = UNKNOWN
        self.assertTrue(rpc.contains_unknowns(wide_dict))

        wide_list = list(range(1000))
        self.assertFalse(rpc.contains_unknowns(wide_list))

        wide_list[500] = UNKNOWN
        self.assertTrue(rpc.contains_unknowns(wide_list))

    def test_pulumi_structure(self):
        resource_props = {
            "name": "my-bucket",
            "arn": "arn:aws:s3:::my-bucket",
            "region": "us-west-2",
            "tags": {
                "Environment": "production",
                "Team": "platform",
            },
            "versioning": {
                "enabled": True,
                "mfaDelete": False,
            },
            "cors": [
                {
                    "allowedHeaders": ["*"],
                    "allowedMethods": ["GET", "POST"],
                    "allowedOrigins": ["https://example.com"],
                    "maxAgeSeconds": 3600,
                }
            ],
        }
        self.assertFalse(rpc.contains_unknowns(resource_props))

        resource_props_preview = resource_props.copy()
        resource_props_preview["arn"] = UNKNOWN
        self.assertTrue(rpc.contains_unknowns(resource_props_preview))
