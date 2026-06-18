import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
num_items = config.require_int("numItems")
item_list = config.require_object("itemList")
num_resource: list[Any] = []
for range in [{"value": i} for i in range(0, num_items)]:
    num_resource.append(nestedobject.Target(f"numResource-{range['value']}", name=f"num-{range['value']}"))
num_target = nestedobject.Target("numTarget", name=num_resource[0].name.apply(lambda name: f"{name}+"))
list_resource: list[Any] = []
for range in [{"key": k, "value": v} for [k, v] in enumerate(item_list)]:
    list_resource.append(nestedobject.Target(f"listResource-{range['key']}", name=f"{range['key']}:{range['value']}"))
list_target = nestedobject.Target("listTarget", name=list_resource[1].name.apply(lambda name: f"{name}+"))
list_dyn_target: list[Any] = []
for range in [{"key": k, "value": v} for [k, v] in enumerate(item_list)]:
    list_dyn_target.append(nestedobject.Target(f"listDynTarget-{range['key']}", name=list_resource[range["key"]].name.apply(lambda name: f"{name}!")))
