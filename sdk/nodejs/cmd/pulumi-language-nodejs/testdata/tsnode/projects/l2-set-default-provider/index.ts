import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const simple_provider = new simple.Provider("simple_provider", {});
pulumi.withDefaultProviders([simple_provider], () => {
    const non_default_resource = new simple.Resource("non_default_resource", {value: true});
});
const default_resource = new simple.Resource("default_resource", {value: true});
