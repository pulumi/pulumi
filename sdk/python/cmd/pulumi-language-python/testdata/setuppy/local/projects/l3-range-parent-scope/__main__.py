import pulumi
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
prefix = config.require("prefix")
item = []
for range in [{"value": i} for i in range(0, 2)]:
    item.append(nestedobject.Target(f"item-{range['value']}", name=f"{prefix}-{range['value']}"))
