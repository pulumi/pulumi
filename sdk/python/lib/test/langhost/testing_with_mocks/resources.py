import pulumi
from pulumi import Output

class MyComponent(pulumi.ComponentResource):
    outprop: pulumi.Output[str]
    def __init__(self, name, inprop: pulumi.Input[str] = None, opts = None):
        super().__init__('pkg:index:MyComponent', name, None, opts)
        if inprop is None:
                raise TypeError("Missing required property 'inprop'")
        self.outprop = pulumi.Output.from_input(inprop).apply(lambda x: f"output: {x}")

class Instance(pulumi.CustomResource):
    public_ip: pulumi.Output[str]
    def __init__(self, resource_name, name: pulumi.Input[str] = None, opts = None):
        if name is None:
                raise TypeError("Missing required property 'name'")
        __props__: dict = dict()
        __props__["public_ip"] = None
        __props__["name"] = name
        super(Instance, self).__init__('aws:ec2/instance:Instance', resource_name, __props__, opts)

mycomponent = MyComponent("mycomponent", inprop="hello")
myinstance = Instance("instance", name="myvm")
