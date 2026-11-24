import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
item_map = config.require_object("itemMap")
map_resource: dict[str, nestedobject.Target] = {}
for map_resource_range in [{"key": k, "value": v} for [k, v] in sorted((item_map).items())]:
    map_resource[map_resource_range['key']] = nestedobject.Target(f"mapResource-{map_resource_range['key']}", name=f"{map_resource_range['key']}={map_resource_range['value']}")
map_target = nestedobject.Target("mapTarget", name=map_resource["k1"].name.apply(lambda name: f"{name}+"))
