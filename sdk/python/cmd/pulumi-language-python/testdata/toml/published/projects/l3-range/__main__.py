import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
num_items = config.require_int("numItems")
item_list = config.require_object("itemList")
item_map = config.require_object("itemMap")
create_bool = config.require_bool("createBool")
num_resource: list[nestedobject.Target] = []
for num_resource_range in [{"value": i} for i in range(0, num_items)]:
    num_resource.append(nestedobject.Target(f"numResource-{num_resource_range['value']}", name=f"num-{num_resource_range['value']}"))
list_resource: list[nestedobject.Target] = []
for list_resource_range in [{"key": k, "value": v} for [k, v] in enumerate(item_list)]:
    list_resource.append(nestedobject.Target(f"listResource-{list_resource_range['key']}", name=f"{list_resource_range['key']}:{list_resource_range['value']}"))
map_resource: dict[str, nestedobject.Target] = {}
for map_resource_range in [{"key": k, "value": v} for [k, v] in sorted((item_map).items())]:
    map_resource[map_resource_range['key']] = nestedobject.Target(f"mapResource-{map_resource_range['key']}", name=f"{map_resource_range['key']}={map_resource_range['value']}")
bool_resource = None
if create_bool:
    bool_resource = nestedobject.Target("boolResource", name="bool-resource")
