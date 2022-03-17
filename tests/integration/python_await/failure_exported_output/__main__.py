import asyncio
import pulumi

exported = pulumi.Output.from_input(asyncio.sleep(1, "foo"))
pulumi.export("exported", exported)
exported.apply(print)

printed = pulumi.Output.from_input(asyncio.sleep(2, "printed"))
printed.apply(print)

not_printed = pulumi.Output.from_input(asyncio.sleep(4, "not printed"))
not_printed.apply(print)

output = pulumi.Output.from_input(asyncio.sleep(3, []))
output.apply(lambda x: x[0])
