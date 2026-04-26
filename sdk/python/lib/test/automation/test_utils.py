# Copyright 2016, Pulumi Corporation.
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
import uuid
from contextlib import contextmanager

from pulumi.automation import Stack, fully_qualified_stack_name


@contextmanager
def stack_cleanup(stack: Stack, destroy: bool = True):
    """Context manager that ensures a stack is destroyed and removed after use.

    Usage:
        stack = create_stack(stack_name, work_dir=work_dir)
        with stack_cleanup(stack):
            stack.up()
            # ... assertions ...

    Set destroy=False to skip the destroy step.
    """
    try:
        yield stack
    finally:
        try:
            if destroy:
                stack.destroy()
        finally:
            stack.workspace.remove_stack(stack.name, force=True)


def get_test_org():
    env_var = os.getenv("PULUMI_TEST_ORG")
    if env_var is not None:
        return env_var
    if os.getenv("PULUMI_ACCESS_TOKEN") is None:
        return "organization"
    test_org = "moolumi"
    return test_org


def get_test_suffix() -> str:
    return str(uuid.uuid4())


def stack_namer(project_name: str) -> str:
    return fully_qualified_stack_name(
        get_test_org(), project_name, f"int_test_{get_test_suffix()}"
    )
