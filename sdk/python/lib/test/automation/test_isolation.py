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

# Regresses [pulumi/pulumi#8633]: sequential operations like
# `stack.up` with inline programs should be isolated from each other,
# so that errors from the first operation do not infect the subsequent
# operations.

import pytest
import typing
import uuid

import pulumi
from pulumi import automation


class BadResource(pulumi.CustomResource):
    def __init__(self,
                 resource_name: str,
                 opts: typing.Optional[pulumi.ResourceOptions] = None):
        if opts is None:
            opts = pulumi.ResourceOptions()
        super().__init__("badprovider::BadResource", resource_name, {}, opts)


def program():
    config = pulumi.Config()
    bad = config.get_int('bad') or 0
    if bad == 1:
        BadResource('bad_resource')


def ignore(*args, **kw):
    pass


def test_isolation():
    stack = automation.create_stack(
        stack_name=f'isolation-test-{uuid.uuid4()}',
        project_name='isolation-test',
        program=program)

    with pytest.raises(automation.errors.CommandError):
        stack.set_config('bad', automation.ConfigValue('1'))
        stack.up(on_output=ignore)

    stack.set_config('bad', automation.ConfigValue('0'))
    stack.up(on_output=ignore)
