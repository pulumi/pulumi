import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const withDefaultURL = new simple.Resource("withDefaultURL", {value: true});
const withExplicitDefaultURL = new simple.Resource("withExplicitDefaultURL", {value: true});
const withCustomURL1 = new simple.Resource("withCustomURL1", {value: true}, {
    pluginDownloadURL: "https://custom.pulumi.test/provider1",
});
const withCustomURL2 = new simple.Resource("withCustomURL2", {value: false}, {
    pluginDownloadURL: "https://custom.pulumi.test/provider2",
});
