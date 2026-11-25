import * as pulumi from "@pulumi/pulumi";
import * as moduleformatenum from "@pulumi/moduleformatenum";

// Enum literals for enum types whose modules have to be resolved with the module format.
const res = new moduleformatenum.mod.Res("res", {
    color: moduleformatenum.mod.Color.Blue,
    shade: moduleformatenum.Shade.Dark,
});
