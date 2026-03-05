import * as pulumi from "@pulumi/pulumi";
import * as config from "@pulumi/config";

const prov = new config.Provider("prov", {
    name: "my config",
    pluginDownloadURL: "not the same as the pulumi resource option",
});
// Note this isn't _using_ the explicit provider, it's just grabbing a value from it.
const res = new config.Resource("res", {text: prov.version});
export const pluginDownloadURL = prov.pluginDownloadURL;
