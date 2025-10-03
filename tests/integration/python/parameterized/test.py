import asyncio

import pulumi
import pulumi.runtime
import pytest


class Mocks(pulumi.runtime.Mocks):
    def call(self, args):
        raise Exception(f"unknown function {args.token}")

    def new_resource(self, args):
        return [f"{args.name}_id", args.inputs]


@pytest.fixture(scope="session")
def event_loop():
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    try:
        yield loop
    finally:
        loop.close()


@pytest.fixture
def resource_mocks(event_loop):
    old_settings = pulumi.runtime.settings.SETTINGS
    try:
        pulumi.runtime.set_mocks(
            Mocks(), project="project", stack="stack", preview=False
        )
        yield
    finally:
        pulumi.runtime.settings.configure(old_settings)


@pulumi.runtime.test
async def test_should_create_random_resource(resource_mocks):
    import pulumi_pkg as pkg

    random = pkg.Random("random", length=8)
    assert random is not None

    result = await random.id.future()
    assert result == "random_id"
