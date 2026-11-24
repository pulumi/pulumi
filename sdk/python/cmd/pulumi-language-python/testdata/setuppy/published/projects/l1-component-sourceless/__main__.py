import pulumi
import json

my_component = pulumi.ComponentResource("my:custom:Component", "myComponent", {"aNumber": 42, "aString": "hello", "aJson": json.dumps({
    "key": "value",
})})
