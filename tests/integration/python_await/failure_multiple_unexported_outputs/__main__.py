import asyncio
import pulumi

output = pulumi.Output.from_input(asyncio.sleep(3, []))
output.apply(lambda x: x[0])

foo = pulumi.Output.from_input(asyncio.sleep(1, "foo"))
foo.apply(print)

printed = pulumi.Output.from_input(asyncio.sleep(2, "printed"))
printed.apply(print)

not_printed = pulumi.Output.from_input(asyncio.sleep(4, "not printed"))
not_printed.apply(print)
