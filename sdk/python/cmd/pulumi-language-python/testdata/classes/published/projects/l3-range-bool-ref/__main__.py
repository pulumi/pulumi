import pulumi
from typing import Any
import pulumi_nestedobject as nestedobject

config = pulumi.Config()
create_bool = config.require_bool("createBool")
bool_resource = None
if create_bool:
    bool_resource = nestedobject.Target("boolResource", name="bool-resource")
bool_target = nestedobject.Target("boolTarget", name=bool_resource.name.apply(lambda name: f"{name}+") if bool_resource is not None else None)
