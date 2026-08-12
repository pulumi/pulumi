import random
import string
from pulumi.provider.experimental.provider import DiffRequest, Provider, CreateResponse, DiffResponse, CheckResponse, ReadResponse
from pulumi.provider.experimental.property_value import PropertyValue

class RandomProvider(Provider):
    """
    A provider for a resource that generates a random string of a given length with an optional prefix.
    """

    schema = {
        "resources": {
            "testprovider:index:Random": {
                "description": "A test resource that generates a random string of a given length and with an optional prefix.",
                "type": "object",
                "properties": {
                    "length": {
                        "type": "integer",
                        "description": "The length of the random string (not including the prefix, if any).",
                    },
                    "prefix": {
                        "type": "string",
                        "description": "An optional prefix.",
                    },
                    "result": {
                        "type": "string",
                        "description": "A random string.",
                    },
                },
                "inputProperties": {
                    "length": {
                        "type": "integer",
                        "description": "The length of the random string (not including the prefix, if any).",
                    },
                    "prefix": {
                        "type": "string",
                        "description": "An optional prefix.",
                    },
                },
            }
        }
    }

    async def check(self, request):
        inputs = request.new_inputs
        if "length" not in inputs:
            raise ValueError("Missing required property 'length'")
        if "prefix" not in inputs:
            inputs["prefix"] = PropertyValue("")

        length = inputs.get("length").value
        if not isinstance(length, float):
            raise ValueError(f"Expected 'length' to be a number, got {type(length)}")

        prefix = inputs.get("prefix").value
        if not isinstance(prefix, str):
            raise ValueError(f"Expected 'prefix' to be a string, got {type(prefix)}")

        return CheckResponse(inputs=inputs)

    async def diff(self, request: DiffRequest):
        replaces = []
        if request.old_inputs.get("length") != request.new_inputs.get("length"):
            replaces.append("length")
        if request.old_inputs.get("prefix") != request.new_inputs.get("prefix"):
            replaces.append("prefix")

        return DiffResponse(
            changes=bool(replaces),
            replaces=replaces,
        )

    async def create(self, request):
        length = request.properties.get("length").value
        prefix = request.properties.get("prefix").value

        result = self._make_random(int(length))
        full_result = f"{prefix}{result}"

        return CreateResponse(
            resource_id=full_result,
            properties={
                "length": PropertyValue(length),
                "prefix": PropertyValue(prefix),
                "result": PropertyValue(full_result),
            },
        )

    async def delete(self, request):
        pass

    @staticmethod
    def _make_random(length: int) -> str:
        """
        Generates a random string of the given length.
        """
        charset = string.ascii_letters + string.digits
        return ''.join(random.choice(charset) for _ in range(length))