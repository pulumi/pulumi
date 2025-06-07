import pulumi
import pulumi.runtime
import unittest

class Mocks(pulumi.runtime.Mocks):
    def call(self, args):
        raise Exception(f"unknown function {args['token']}")

    def new_resource(self, args):
        return [f"{args['name']}_id", args['inputs']]

pulumi.runtime.set_mocks(Mocks(), project="project", stack="stack", preview=False)

class TestPackage(unittest.TestCase):
    def test_should_create_random_resource(self):
        import pulumi_pkg as pkg  # Assuming 'pkg' is the generated Python SDK for your provider
        random = pkg.Random("random", length=8)
        assert random is not None