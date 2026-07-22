import pulumi
import pulumi_extra_package_names as extra_package_names

prov = extra_package_names.Provider("prov")
via_provider = extra_package_names.mod.Res("viaProvider",
    choice=extra_package_names.mod.Choice.FIRST,
    obj=extra_package_names.mod.ObjArgs(
        label="explicit",
        choice=extra_package_names.mod.Choice.SECOND,
    ),
    opts = pulumi.ResourceOptions(provider=prov))
via_package = extra_package_names.mod.Res("viaPackage",
    choice=extra_package_names.mod.Choice.SECOND,
    obj=extra_package_names.mod.ObjArgs(
        label="bare",
        choice=extra_package_names.mod.Choice.FIRST,
    ))
thing = extra_package_names.mod.get_thing_output(text="hello")
pulumi.export("result", thing.result)
