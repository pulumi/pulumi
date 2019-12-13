from pulumi import ComponentResource


class MyResource(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("my:module:Resource", name, None, opts)


MyResource("testResource1")
