import pulumi

my_component = pulumi.ComponentResource("my:custom:Component", "myComponent", {"aNumber": 42, "aString": "hello"})
