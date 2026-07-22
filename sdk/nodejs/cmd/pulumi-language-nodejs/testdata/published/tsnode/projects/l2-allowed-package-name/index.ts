import * as pulumi from "@pulumi/pulumi";
import * as extra_package_names from "@pulumi/extra-package-names";

const prov = new extra_package_names.Provider("prov", {});
const viaProvider = new extra_package_names.mod.Res("viaProvider", {
    choice: extra_package_names.mod.Choice.First,
    obj: {
        label: "explicit",
        choice: extra_package_names.mod.Choice.Second,
    },
}, {
    provider: prov,
});
const viaPackage = new extra_package_names.mod.Res("viaPackage", {
    choice: extra_package_names.mod.Choice.Second,
    obj: {
        label: "bare",
        choice: extra_package_names.mod.Choice.First,
    },
});
const thing = extra_package_names.mod.getThingOutput({
    text: "hello",
});
export const result = thing.result;
