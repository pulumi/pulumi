import asyncio
import pulumi

output = pulumi.Output.from_input(asyncio.sleep(1, []))
output.apply(lambda x: x[0])
