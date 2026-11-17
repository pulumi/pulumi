import * as pulumi from "@pulumi/pulumi";
import * as config_enum from "@pulumi/config-enum";

const prov = new config_enum.Provider("prov", {
    aString: "hello",
    aEnum: config_enum.MyEnum.Two,
});
// Reference the provider's outputs - including the enum - from another resource.
const res = new config_enum.Resource("res", {
    theString: prov.aString,
    theEnum: prov.aEnum,
}, {
    provider: prov,
});
