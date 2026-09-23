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

import pytest_asyncio

import pulumi
from pulumi import delivery
from pulumi.runtime import mocks

APP_OUTPUTS = {"url": "https://app.example.com"}


def db_outputs():
    return {"endpoint": "db.internal:5432", "password": pulumi.Output.secret("hunter2")}


class DeliveryMocks(pulumi.runtime.Mocks):
    def __init__(self):
        self.registered = []

    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        self.registered.append(args)
        if args.typ == "pulumi:delivery:Stack":
            return [
                args.name + "_id",
                {
                    "stack": args.inputs["stack"],
                    "outputs": db_outputs(),
                    "secretOutputNames": ["password"],
                },
            ]
        if args.typ == "pulumi:delivery:StackGroup":
            return [
                args.name + "_id",
                {
                    "members": [
                        {
                            "stack": "org/db/prod",
                            "outputs": db_outputs(),
                            "secretOutputNames": ["password"],
                        },
                        {
                            "stack": "org/app/prod",
                            "outputs": APP_OUTPUTS,
                            "secretOutputNames": [],
                        },
                    ]
                },
            ]
        raise AssertionError(f"unknown type {args.typ}")

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}


@pytest_asyncio.fixture
async def delivery_mocks():
    mock = DeliveryMocks()
    mocks.set_mocks(mock)
    yield mock


@pulumi.runtime.test
async def test_stack(delivery_mocks):
    db = delivery.Stack("db", stack="org/db/prod", source={"directory": "infra/db"})

    assert await db.stack.future() == "org/db/prod"
    assert delivery_mocks.registered[0].inputs == {
        "stack": "org/db/prod",
        "source": {"directory": "infra/db"},
    }

    endpoint = db.get_output("endpoint")
    assert await endpoint.future() == "db.internal:5432"
    assert not await endpoint.is_secret()

    password = db.require_output("password")
    assert await password.future() == "hunter2"
    assert await password.is_secret()

    assert await db.get_output("missing").future() is None


@pulumi.runtime.test
async def test_stack_group(delivery_mocks):
    group = delivery.StackGroup(
        "services",
        stacks=[
            {"stack": "org/db/prod", "source": {"directory": "infra/db"}},
            {"stack": "org/app/prod", "source": {"directory": "infra/app"}},
        ],
    )

    url = group.get_output("org/app/prod", "url")
    assert await url.future() == "https://app.example.com"
    assert not await url.is_secret()
    assert await group.require_output("org/db/prod", "password").is_secret()
    assert await group.stack_outputs("org/app/prod").future() == APP_OUTPUTS
