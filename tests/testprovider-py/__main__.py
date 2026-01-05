import json
import sys
import base64

from pulumi.provider.experimental.provider import(
    Provider, InvokeResponse, CreateResponse, GetSchemaResponse,
    ParameterizeResponse,ParametersArgs, ParametersValue
)
from pulumi.provider.experimental.server import main as server_main

from src import FailsOnDeleteProvider, RandomProvider, FailsOnCreateProvider, EchoProvider

# Constants
PROVIDER_NAME = "testprovider"
VERSION = "0.0.1"

# Provider schema
provider_schema = {
    "name": PROVIDER_NAME,
    "version": VERSION,
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
        "testprovider:index:Echo":          EchoProvider(),
}

def merge(a: dict, b: dict, path=[]) -> dict:
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
        self.parameter = "testprovider"
        self.version = version

    async def parameterize(self, request) -> ParameterizeResponse:
        if isinstance(request.parameters, ParametersArgs):
            args = request.parameters.args
            if len(args) != 1:
                raise ValueError("expected exactly one argument")
            self.parameter = args[0]
        elif isinstance(request.parameters, ParametersValue):
            val = request.parameters.value.decode('utf-8')
            if val == "":
                raise ValueError("expected a non-empty string value")
            self.parameter = val
        else:
            raise ValueError("unexpected parameter type")

        for k in list(testProviders.keys()):
            prov = testProviders[k]
            k = k.replace("testprovider", self.parameter, 1)
            testProviders[k] = prov

        return ParameterizeResponse(name=self.parameter, version=self.version)

    async def get_schema(self, request) -> GetSchemaResponse:
        provider_schema["name"] = self.parameter
        provider_schema["displayName"] = self.parameter

        for k in list(provider_schema["resources"].keys()):
            schema = provider_schema["resources"][k]
            del provider_schema["resources"][k]
            k = k.replace("testprovider", self.parameter)

            for method in list(schema.get("methods", {}).keys()):
                method_schema = schema["methods"][method]
                schema["methods"][method] = method_schema.replace("testprovider", self.parameter)

            provider_schema["resources"][k] = schema

        for k in list(provider_schema["functions"].keys()):
            schema = provider_schema["functions"][k]
            del provider_schema["functions"][k]
            k = k.replace("testprovider", self.parameter)
            provider_schema["functions"][k] = schema

        if self.parameter != PROVIDER_NAME:
            provider_schema["parameterization"] = {
                "baseProvider": {
                    "name": PROVIDER_NAME,
                    "version": self.version,
                },
                "parameter": base64.b64encode(self.parameter.encode('utf-8')).decode('utf-8'),
            }

        return GetSchemaResponse(
            schema=json.dumps(provider_schema),
        )

    async def invoke(self, request) -> InvokeResponse:
        """
        Dynamically executes a built-in function in the provider.
        """
        if request.tok == "testprovider:index:returnArgs".replace("testprovider", self.parameter):
            return InvokeResponse(return_value=request.args)
        if request.tok == "testprovider:index:doEcho".replace("testprovider", self.parameter):
            if "echo" not in request.args:
                raise Exception("Missing 'echo' argument")
            return InvokeResponse(return_value={"echo": request.args["echo"]})
        
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
    
    async def call(self, request):
        if request.tok == "testprovider:index:Echo/doEchoMethod".replace("testprovider", self.parameter):
            if "echo" not in request.args:
                raise Exception("Missing 'echo' argument")
            return InvokeResponse(return_value={"echo": request.args["echo"]})
        
        raise Exception(f"Unknown Call token '{request.tok}'")

def main():
    """
    Entry point for the provider.
    """
    provider = TestProvider(PROVIDER_NAME, VERSION)
    server_main(sys.argv, VERSION, provider)

if __name__ == "__main__":
    main()