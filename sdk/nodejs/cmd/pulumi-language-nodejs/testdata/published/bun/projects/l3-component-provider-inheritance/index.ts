import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import { Local } from "./local";

const explicit = new simple.Provider("explicit", {});
const withProviders = new Local("withProviders", {
    providers: {
        simple: explicit,
    },
});
export const result = withProviders.result;
