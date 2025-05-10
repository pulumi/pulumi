import json
import sys

from pulumi.provider.experimental.provider import Provider, InvokeResponse, CreateResponse, GetSchemaResponse
from pulumi.provider.experimental.server import main as server_main

from src import FailsOnDeleteProvider, RandomProvider, FailsOnCreateProvider

# Constants
PROVIDER_NAME = "testprovider"
VERSION = "0.0.1"

# Provider schema
provider_schema = {
    "name": PROVIDER_NAME,
    "description": "A test provider.",
    "displayName": PROVIDER_NAME,
    "config": {},
    "provider": {
        "description": "The provider type for the testprovider package.",
        "type": "object",
        "inputProperties": {},
    },
    "types": {},
    "resources": {},
    "functions": {},
    "language": {},
}


testProviders = {
		"testprovider:index:Random":        RandomProvider(),
		"testprovider:index:FailsOnDelete": FailsOnDeleteProvider(),
		"testprovider:index:FailsOnCreate": FailsOnCreateProvider(),
}

def merge(a: dict, b: dict, path=[]):
    for key in b:
        if key in a:
            if isinstance(a[key], dict) and isinstance(b[key], dict):
                merge(a[key], b[key], path + [str(key)])
            elif a[key] != b[key]:
                raise Exception('Conflict at ' + '.'.join(path + [str(key)]))
        else:
            a[key] = b[key]
    return a

for name, provider in testProviders.items():
    provider_schema = merge(provider.schema, provider_schema)

class TestProvider(Provider):
    """
    A test provider implementation.
    """

    def __init__(self, name: str, version: str):
        super().__init__()
        self.parameter = None

    async def get_schema(self, request) -> GetSchemaResponse:
        return GetSchemaResponse(
            schema=json.dumps(provider_schema),
        )

    async def invoke(self, request) -> InvokeResponse:
        """
        Dynamically executes a built-in function in the provider.
        """
        if request.tok == "testprovider:index:returnArgs":
            return InvokeResponse(return_value=request.args)
        raise Exception(f"Unknown Invoke token '{request.tok}'")

    async def create(self, request) -> CreateResponse:
        provider = testProviders.get(request.type)
        if provider is None:
            raise Exception(f"Unknown resource type '{request.type}'")
        return await provider.create(request)

    async def check(self, request):
        provider = testProviders.get(request.type)
        if provider is None:
            raise Exception(f"Unknown resource type '{request.type}'")
        return await provider.check(request)

    async def delete(self, request):
        provider = testProviders.get(request.type)
        if provider is None:
            raise Exception(f"Unknown resource type '{request.type}'")
        return await provider.delete(request)

    async def read(self, request):
        provider = testProviders.get(request.type)
        if provider is None:
            raise Exception(f"Unknown resource type '{request.type}'")
        return await provider.read(request)

def main():
    """
    Entry point for the provider.
    """
    provider = TestProvider(PROVIDER_NAME, VERSION)
    server_main(sys.argv, VERSION, provider)

if __name__ == "__main__":
    main()