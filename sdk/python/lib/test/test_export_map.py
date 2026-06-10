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

"""Tests for get_current_export_map, which exposes stack exports for
unit testing assertions."""

from copy import deepcopy

from pulumi.runtime.stack import Stack
from pulumi.runtime import settings, mocks
import pulumi


class MyMocks(pulumi.runtime.Mocks):
    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        return [args.name + "_id", args.inputs]

    def call(self, args: pulumi.runtime.MockCallArgs):
        raise Exception("call")


def _setup_mocks():
    """Reset options with proper project/stack names and configure a mock
    monitor so that Stack URNs are well-formed, without auto-creating a
    root Stack (which ``set_mocks`` would do)."""
    old_settings = deepcopy(settings.SETTINGS)
    mm = MyMocks()
    settings.reset_options(project="test", stack="test")
    settings.configure(
        mocks.MockSettings(
            monitor=mocks.MockMonitor(mm),
            engine=mocks.MockEngine(None),
            project="test",
            stack="test",
            dry_run=False,
        )
    )
    return old_settings


def test_get_current_export_map():
    old_settings = _setup_mocks()

    def program():
        pulumi.export("fruit", "banana")
        pulumi.export("color", "yellow")

    try:
        Stack(program)

        export_map = pulumi.get_current_export_map()
        assert export_map == {"fruit": "banana", "color": "yellow"}
    finally:
        settings.configure(old_settings)


def test_get_current_export_map_returns_copy():
    old_settings = _setup_mocks()

    def program():
        pulumi.export("key", "value")

    try:
        Stack(program)

        export_map = pulumi.get_current_export_map()
        export_map["key"] = "modified"

        # The original should be unaffected.
        assert pulumi.get_current_export_map() == {"key": "value"}
    finally:
        settings.configure(old_settings)


def test_get_current_export_map_empty():
    old_settings = _setup_mocks()

    def program():
        pass

    try:
        Stack(program)

        export_map = pulumi.get_current_export_map()
        assert export_map == {}
    finally:
        settings.configure(old_settings)
