import pulumi
import pulumi_module_format as module_format
import pulumi_names as names

config = pulumi.Config()
names_1 = config.get_bool("names")
if names_1 is None:
    names_1 = True
names_2 = config.get_bool("Names")
if names_2 is None:
    names_2 = True
mod = config.get("mod")
if mod is None:
    mod = "module"
mod_1 = config.get("Mod")
if mod_1 is None:
    mod_1 = "format"
names_resource = names.mod.Res("namesResource", value=names_1)
mod_resource = module_format.mod.Resource("modResource", text=f"{mod}-{mod_1}")
pulumi.export("namesResourceVal", names_resource.value)
pulumi.export("modResourceText", mod_resource.text)
pulumi.export("nameVariables", names_1 and names_2)
pulumi.export("modVariables", f"{mod}-{mod_1}")
