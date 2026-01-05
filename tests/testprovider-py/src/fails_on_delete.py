from pulumi.provider.experimental.provider import Provider, CreateResponse


class FailsOnDeleteProvider(Provider):

    schema = {
        "resources": {
            "testprovider:index:FailsOnDelete": {
                "description": "A test resource that fails on delete.",
                "type": "object",
            }
        }
	}

    def __init__(self):
        self.id = 0

    async def create(self, request):
        self.id += 1
        return CreateResponse(
            resource_id=str(self.id),
        )

    async def delete(self, request):
        raise Exception("Delete always fails for the FailsOnDelete resource.")
