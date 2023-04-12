# Copyright 2016-2022, Pulumi Corporation.
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
from random import random

from pulumi.automation import fully_qualified_stack_name

def get_test_org():
    test_org = "pulumi-test"
    env_var = os.getenv("PULUMI_TEST_ORG")
    if env_var is not None:
        test_org = env_var
    return test_org


def get_test_suffix() -> int:
    return int(100000 + random() * 900000)


def stack_namer(project_name):
    return fully_qualified_stack_name(get_test_org(), project_name, f"int_test_{get_test_suffix()}")
