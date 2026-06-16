import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
prefix = config.require("prefix")
item: list[Any] = []
item_range: list[dict[str, Any]] = [{"value": i} for i in range(0, 2)]
for range in item_range:
    item.append(nestedobject.Target(f"item-{range['value']}", name=f"{prefix}-{range['value']}"))
