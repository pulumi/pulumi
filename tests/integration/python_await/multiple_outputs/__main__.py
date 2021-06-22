import asyncio
import pulumi

output = pulumi.Output.from_input(asyncio.sleep(3, "magic string"))
output.apply(print)

exported = pulumi.Output.from_input(asyncio.sleep(2, "foo"))
pulumi.export("exported", exported)
exported.apply(print)

another = pulumi.Output.from_input(asyncio.sleep(5, "bar"))
another.apply(print)


