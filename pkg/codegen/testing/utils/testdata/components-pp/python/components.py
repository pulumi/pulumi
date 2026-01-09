import pulumi
from another_component import AnotherComponent
from exampleComponent import ExampleComponent
from simpleComponent import SimpleComponent

simple_component = SimpleComponent("simpleComponent")
multiple_simple_components = []
for range in [{"value": i} for i in range(0, 10)]:
    multiple_simple_components.append(SimpleComponent(f"multipleSimpleComponents-{range['value']}"))
another_component = AnotherComponent("anotherComponent")
example_component = ExampleComponent("exampleComponent", {
    'input': "doggo", 
    'ipAddress': [
        127,
        0,
        0,
        1,
    ], 
    'cidrBlocks': {
        "one": "uno",
        "two": "dos",
    }, 
    'githubApp': {
        "id": "example id",
        "keyBase64": "base64 encoded key",
        "webhookSecret": "very important secret",
    }, 
    'servers': [
        {
            "name": "First",
        },
        {
            "name": "Second",
        },
    ], 
    'deploymentZones': {
        "first": {
            "zone": "First zone",
        },
        "second": {
            "zone": "Second zone",
        },
    }})
pulumi.export("result", example_component.result)
