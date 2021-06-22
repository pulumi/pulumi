import asyncio
import pulumi

output = pulumi.Output.from_input(asyncio.sleep(1, "magic string"))
output.apply(print)
