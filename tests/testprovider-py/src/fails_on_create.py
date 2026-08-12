from pulumi.provider.experimental.provider import Provider


class FailsOnCreateProvider(Provider):
    schema = {
        "resources": {
            "testprovider:index:FailsOnCreate": {
                "description": "A test resource that fails on create.",
                "type": "object",
            }
        }
    }

    def __init__(self):
        self.id = 0

    async def create(self, request):
        raise Exception("Create always fails for the FailsOnCreate resource.")
