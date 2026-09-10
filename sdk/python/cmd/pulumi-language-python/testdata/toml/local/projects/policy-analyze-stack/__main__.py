import pulumi
from typing import Any
import pulumi_simple as simple

config = pulumi.Config()
extra_count = config.require_int("extraCount")
res1 = simple.Resource("res1", value=True)
res2: list[simple.Resource] = []
for res2_range in [{"value": i} for i in range(0, extra_count)]:
    res2.append(simple.Resource(f"res2-{res2_range['value']}", value=False))
