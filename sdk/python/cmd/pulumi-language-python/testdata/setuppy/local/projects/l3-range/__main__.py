import pulumi
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
num_items = config.require_int("numItems")
item_list = config.require_object("itemList")
item_map = config.require_object("itemMap")
create_bool = config.require_bool("createBool")
num_resource = []
for range in [{"value": i} for i in range(0, num_items)]:
    num_resource.append(nestedobject.Target(f"numResource-{range['value']}", name=f"num-{range['value']}"))
list_resource = []
for range in [{"key": k, "value": v} for [k, v] in enumerate(item_list)]:
    list_resource.append(nestedobject.Target(f"listResource-{range['key']}", name=f"{range['key']}:{range['value']}"))
map_resource = []
for range in [{"key": k, "value": v} for [k, v] in sorted((item_map).items())]:
    map_resource.append(nestedobject.Target(f"mapResource-{range['key']}", name=f"{range['key']}={range['value']}"))
bool_resource = None
if create_bool:
    bool_resource = nestedobject.Target("boolResource", name="bool-resource")
