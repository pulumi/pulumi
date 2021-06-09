# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from typing import Any, Optional, Sequence
import uuid

from pulumi import Input, Output, ResourceOptions
import pulumi
import pulumi.dynamic as dyn


class XInputs(object):
    x: Input[str]

    def __init__(self, x):
        self.x = x


class XProvider(dyn.ResourceProvider):
    def create(self, args):
        outs = {**args}
        outs['x'] = {'x': f'{outs["x"]}!'}  # intentional bug changing the type
        return dyn.CreateResult(f'schema-{uuid.uuid4()}', outs=outs)


class X(dyn.Resource):
    x: Output[str]

    def __init__(self, name: str, args: XInputs, opts = None):
        super().__init__(XProvider(), name, vars(args), opts)


x = X(name='my_x', args=XInputs('my_x_value'))
pulumi.export('x', x.x)
