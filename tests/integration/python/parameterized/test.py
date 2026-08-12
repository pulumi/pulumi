import pulumi
import pulumi.runtime
import unittest


class Mocks(pulumi.runtime.Mocks):
    def call(self, args):
        raise Exception(f"unknown function {args.token}")

    def new_resource(self, args):
        return [f"{args.name}_id", args.inputs]


class TestPackage(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        pulumi.runtime.set_mocks(
            Mocks(), project="project", stack="stack", preview=False
        )
        await pulumi.runtime.settings._load_monitor_feature_support()

    async def test_should_create_random_resource(self):
        import pulumi_pkg as pkg

        random = pkg.Random("random", length=8)
        assert random is not None

        result = await random.id.future()
        assert result == "random_id"
