import pulumi
import pulumi_moduleformatenum as moduleformatenum

# Enum literals for enum types whose modules have to be resolved with the module format.
res = moduleformatenum.mod.Res("res",
    color=moduleformatenum.mod.Color.BLUE,
    shade=moduleformatenum.Shade.DARK)
