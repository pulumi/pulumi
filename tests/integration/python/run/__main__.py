# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import pulumi
import asyncio

async def main():
    # sanity check that async really is working
    await asyncio.sleep(0.1)
    pulumi.export("export", "hello")

pulumi.run(main)