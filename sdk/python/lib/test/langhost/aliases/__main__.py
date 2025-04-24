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
from pulumi import Output, ResourceOptions
from pulumi.resource import Alias, CustomResource
from pulumi.runtime.resource import all_aliases


class MyResource(CustomResource):
    def __init__(self, name, opts=None):
        CustomResource.__init__(self, "test:resource:type", name, props={}, opts=opts)


test_cases = [
    {
        "resource_name": "myres1",
        "parent_aliases": [],
        "child_aliases": [],
        "results": [],
    },
    {
        "resource_name": "myres2",
        "parent_aliases": [],
        "child_aliases": [Alias(type_="test:resource:child2")],
        "results": [
            "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres2-child"
        ],
    },
    {
        "resource_name": "myres3",
        "parent_aliases": [],
        "child_aliases": [Alias(name="child2")],
        "results": [
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::child2"
        ],
    },
    {
        "resource_name": "myres4",
        "parent_aliases": [Alias(type_="test:resource:type3")],
        "child_aliases": [Alias(name="myres4-child2")],
        "results": [
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres4-child2",
            "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres4-child",
            "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres4-child2",
        ],
    },
    {
        "resource_name": "myres5",
        "parent_aliases": [Alias(name="myres52")],
        "child_aliases": [Alias(name="myres5-child2")],
        "results": [
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres5-child2",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres52-child",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres52-child2",
        ],
    },
    {
        "resource_name": "myres6",
        "parent_aliases": [
            Alias(name="myres62"),
            Alias(type_="test:resource:type3"),
            Alias(name="myres63"),
        ],
        "child_aliases": [
            Alias(name="myres6-child2"),
            Alias(type_="test:resource:child2"),
        ],
        "results": [
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres6-child2",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres6-child",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres62-child",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres62-child2",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres62-child",
            "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres6-child",
            "urn:pulumi:stack::project::test:resource:type3$test:resource:child::myres6-child2",
            "urn:pulumi:stack::project::test:resource:type3$test:resource:child2::myres6-child",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres63-child",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child::myres63-child2",
            "urn:pulumi:stack::project::test:resource:type$test:resource:child2::myres63-child",
        ],
    },
]

for test_case in test_cases:
    resource_name: str = test_case["resource_name"]
    parent_aliases = test_case["parent_aliases"]
    child_aliases = test_case["child_aliases"]
    results = test_case["results"]
    res = MyResource(resource_name, ResourceOptions(aliases=parent_aliases))
    aliases = all_aliases(
        child_aliases, resource_name + "-child", "test:resource:child", res
    )
    print(len(aliases))
    if len(aliases) != len(results):
        raise Exception(f"expected ${len(results)} aliases but got ${len(aliases)}")
    for i in range(len(aliases)):
        result = results[i]

        def validate(alias_urn: str, result=result):
            print(f"validating {alias_urn}")
            if alias_urn != result:
                raise Exception(f"expected {result} but got {alias_urn}")

        Output.from_input(aliases[i]).apply(validate)
