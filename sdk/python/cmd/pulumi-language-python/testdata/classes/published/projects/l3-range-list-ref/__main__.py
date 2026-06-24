import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
num_items = config.require_int("numItems")
item_list = config.require_object("itemList")
num_resource: list[nestedobject.Target] = []
for num_resource_range in [{"value": i} for i in range(0, num_items)]:
    num_resource.append(nestedobject.Target(f"numResource-{num_resource_range['value']}", name=f"num-{num_resource_range['value']}"))
num_target = nestedobject.Target("numTarget", name=num_resource[0].name.apply(lambda name: f"{name}+"))
list_resource: list[nestedobject.Target] = []
for list_resource_range in [{"key": k, "value": v} for [k, v] in enumerate(item_list)]:
    list_resource.append(nestedobject.Target(f"listResource-{list_resource_range['key']}", name=f"{list_resource_range['key']}:{list_resource_range['value']}"))
list_target = nestedobject.Target("listTarget", name=list_resource[1].name.apply(lambda name: f"{name}+"))
list_dyn_target: list[nestedobject.Target] = []
for list_dyn_target_range in [{"key": k, "value": v} for [k, v] in enumerate(item_list)]:
    list_dyn_target.append(nestedobject.Target(f"listDynTarget-{list_dyn_target_range['key']}", name=list_resource[list_dyn_target_range["key"]].name.apply(lambda name: f"{name}!")))
