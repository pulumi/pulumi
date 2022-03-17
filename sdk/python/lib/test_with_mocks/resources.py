# Copyright 2016-2021, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from typing import Optional

import pulumi


class MyComponent(pulumi.ComponentResource):
    outprop: pulumi.Output[str]
    def __init__(self, name, inprop: pulumi.Input[str] = None, opts = None):
        super().__init__("pkg:index:MyComponent", name, None, opts)
        if inprop is None:
            raise TypeError("Missing required property 'inprop'")
        self.outprop = pulumi.Output.from_input(inprop).apply(lambda x: f"output: {x}")


class MyRemoteComponent(pulumi.ComponentResource):
    outprop: pulumi.Output[str]
    def __init__(self, name, inprop: pulumi.Input[str] = None, opts = None):
        if inprop is None:
            raise TypeError("Missing required property 'inprop'")
        __props__: dict = dict()
        __props__["inprop"] = inprop
        __props__["outprop"] = None
        super().__init__("pkg:index:MyRemoteComponent", name, __props__, opts, True)


class Instance(pulumi.CustomResource):
    public_ip: pulumi.Output[str]
    def __init__(self, resource_name, name: pulumi.Input[str] = None, value: pulumi.Input[str] = None, opts = None):
        if opts is None:
            opts = pulumi.ResourceOptions()
        if name is None and not opts.urn:
            raise TypeError("Missing required property 'name'")
        __props__: dict = dict()
        __props__["public_ip"] = None
        __props__["name"] = name
        __props__["value"] = value
        super(Instance, self).__init__("aws:ec2/instance:Instance", resource_name, __props__, opts)


class Module(pulumi.runtime.ResourceModule):
    def version(self):
        return None

    def construct(self, name: str, typ: str, urn: str) -> pulumi.Resource:
        if typ == "aws:ec2/instance:Instance":
            return Instance(name, opts=pulumi.ResourceOptions(urn=urn))
        else:
            raise Exception(f"unknown resource type {typ}")


class MyCustom(pulumi.CustomResource):
    instance: pulumi.Output
    def __init__(self, resource_name, props: Optional[dict] = None, opts = None):
        super(MyCustom, self).__init__("pkg:index:MyCustom", resource_name, props, opts)


def do_invoke():
    value = pulumi.runtime.invoke("test:index:MyFunction", props={"value": 41}).value
    return value["out_value"]


def define_resources():
    mycomponent = MyComponent("mycomponent", inprop="hello")
    myinstance = Instance("instance",
                          name="myvm",
                          value=pulumi.Output.secret("secret_value"))
    mycustom = MyCustom("mycustom", {"instance": myinstance})
    invoke_result = do_invoke()
    myremotecomponent = MyRemoteComponent("myremotecomponent", inprop=myinstance.id.apply(lambda v: f"hello: {v}"))

    # Pass myinstance several more times to ensure deserialization of the resource reference
    # works on other asyncio threads.
    for x in range(5):
        MyCustom(f"mycustom{x}", {"instance": myinstance})

    dns_ref = pulumi.StackReference("dns")

    pulumi.export("hello", "world")
    pulumi.export("outprop", mycomponent.outprop)
    pulumi.export("public_ip", myinstance.public_ip)

    return {
        'mycomponent': mycomponent,
        'myinstance': myinstance,
        'mycustom': mycustom,
        'dns_ref': dns_ref,
        'invoke_result': invoke_result,
        'myremotecomponent': myremotecomponent,
    }


pulumi.runtime.register_resource_module("aws", "ec2/instance", Module())
