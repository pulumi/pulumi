# Copyright 2016, Pulumi Corporation.  All rights reserved.

import pulumi

from component import Component, ComponentArgs, FooArgs, BarArgs


class Dependency(pulumi.CustomResource):
    def __init__(self, name: str):
        super().__init__("testprovider:index:Random", name, {"length": 1})


dep = Dependency("dep")
b = dep.urn.apply(lambda _: "shh")

Component("component", ComponentArgs(
    foo=FooArgs(something="hello"),
    bar=BarArgs(tags={
        "a": "world",
        "b": b,
    })
))
