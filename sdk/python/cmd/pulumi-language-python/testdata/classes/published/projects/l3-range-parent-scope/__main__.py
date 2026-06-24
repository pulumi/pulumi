import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
prefix = config.require("prefix")
item: list[nestedobject.Target] = []
for item_range in [{"value": i} for i in range(0, 2)]:
    item.append(nestedobject.Target(f"item-{item_range['value']}", name=f"{prefix}-{item_range['value']}"))
