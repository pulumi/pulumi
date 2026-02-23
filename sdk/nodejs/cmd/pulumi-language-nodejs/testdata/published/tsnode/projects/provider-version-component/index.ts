import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as simple from "@pulumi/simple";

const simpleV2 = new simple.Provider("simpleV2", {});
const withV22 = new conformance_component.Simple("withV22", {value: true}, {
    providers: {
        simple: simpleV2,
    },
});
const withDefault = new conformance_component.Simple("withDefault", {value: true}, {
    providers: {
        simple: simpleV2,
    },
});
