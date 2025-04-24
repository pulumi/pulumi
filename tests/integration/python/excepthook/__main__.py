# Copyright 2024, Pulumi Corporation.  All rights reserved.

import pulumi
import pulumi_random as random


def transform(args):
    return pulumi.ResourceTransformResult(
        props=args.props,
        opts=args.opts,
    )


random.RandomInteger(
    "test",
    max=999,
    min=100,
    opts=pulumi.ResourceOptions(
        transforms=[transform],
    ),
)
