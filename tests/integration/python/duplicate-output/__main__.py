# Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import pulumi
import asyncio

async def main():
    await asyncio.sleep(1)
    a = pulumi.Output.from_input([1, 2])

    pulumi.export("export1", a)
    pulumi.export("export2", a)

pulumi.run(main)