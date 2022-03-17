# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from pulumi import Output

from component import Component, ComponentArgs, FooArgs, BarArgs

Component("component", ComponentArgs(
    foo=FooArgs(something="hello"),
    bar=BarArgs(tags={
        "a": "world",
        "b": Output.secret("shh"),
    })
))
