import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const prov = new simple.Provider("prov", {}, {
    envVarMappings: {
        MY_VAR: "PROVIDER_VAR",
        OTHER_VAR: "TARGET_VAR",
    },
});
