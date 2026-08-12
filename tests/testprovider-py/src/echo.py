from pulumi.provider.experimental.provider import Provider, CheckResponse, DiffResponse, CreateResponse
from pulumi.provider.experimental.provider import ReadResponse, InvokeResponse, CallResponse

class EchoProvider(Provider):
    """
    A test provider that echoes its input.
    """

    schema = {
        "resources": {
            "testprovider:index:Echo": {
                "description": "A test resource that echoes its input.",
                "type": "object",
                "properties": {
                    "echo": {
                        "type": "string",
                        "description": "Input to echo.",
                    },
                },
                "inputProperties": {
                    "echo": {
                        "type": "string",
                        "description": "An echoed input.",
                    },
                },
                "methods": {
                    "doEchoMethod": "testprovider:index:Echo/doEchoMethod",
                },
            }
        },
        "functions": {
            "testprovider:index:doEcho": {
                "description": "A test invoke that echoes its input.",
                "inputs": {
                    "properties": {
                        "echo": {"type": "string"},
                    }
                },
                "outputs": {
                    "properties": {
                        "echo": {"type": "string"},
                    }
                },
            },
            "testprovider:index:Echo/doEchoMethod": {
                "description": "A test call that echoes its input.",
                "inputs": {
                    "properties": {
                        "__self__": {"$ref": "#/types/testprovider:index:Echo"},
                        "echo": {"type": "string"},
                    }
                },
                "outputs": {
                    "properties": {
                        "echo": {"type": "string"},
                    }
                },
            },
        }
    }

    def __init__(self):
        self._id = 0

    async def check(self, request):
        return CheckResponse(inputs=request.new_inputs)

    async def diff(self, request):
        olds = request.old_state
        news = request.new_inputs
        replaces = []
        changes = False
        if olds.get("echo") != news.get("echo"):
            replaces.append("echo")
            changes = True
        return DiffResponse(changes=changes, replaces=replaces)

    async def create(self, request):
        self._id += 1
        # Echo the input properties as output properties
        return CreateResponse(
            resource_id=str(self._id),
            properties=request.properties,
        )

    async def read(self, request):
        return ReadResponse(
            resource_id=request.resource_id,
            properties=request.properties,
        )

    async def update(self, request):
        raise NotImplementedError("Update not implemented")

    async def delete(self, request):
        return
