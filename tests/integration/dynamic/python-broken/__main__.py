# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

from typing import Any, Optional, Sequence, Dict
import uuid

from pulumi import Input, Output, ResourceOptions
import pulumi
import pulumi.dynamic as dyn


class XInputs(object):
    x: Input[Dict[str, str]]

    def __init__(self, x):
        self.x = x


class XProvider(dyn.ResourceProvider):
    def create(self, args):
        # intentional bug changing the type
        outs = {
            'x': {
                'my_key_1': {
                    'extra_buggy_key': args['x']['my_key_1'] + '!'
                }
            }
        }
        return dyn.CreateResult(f'schema-{uuid.uuid4()}', outs=outs)


class X(dyn.Resource):
    x: Output[Dict[str, str]]

    def __init__(self, name: str, args: XInputs, opts = None):
        super().__init__(XProvider(), name, vars(args), opts)


x = X(name='my_x', args=XInputs({'my_key_1': 'my_value_1'}))
pulumi.export('x', x.x)
