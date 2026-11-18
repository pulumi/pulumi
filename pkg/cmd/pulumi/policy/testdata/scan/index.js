"use strict";
const pulumi = require("@pulumi/pulumi");
const childProcess = require("child_process");

// Shaped like a resource class from a provider SDK.
class Bucket extends pulumi.CustomResource {
    constructor(name, args, opts) {
        super("test:index:Bucket", name, args, opts);
    }
}

const config = new pulumi.Config();
const owner = config.require("owner");

// A data source with a fixture in PulumiScan.yaml, and one without.
const account = pulumi.runtime.invoke("test:index:getAccount", {});
const network = pulumi.runtime.invoke("test:index:getNetwork", {});

// Child processes don't run during a scan.
const commit = childProcess.execSync("git rev-parse HEAD").toString();

const bucket = new Bucket("public", {
    acl: "public-read",
    owner,
    accountId: account.then((a) => a.id),
    commit,
});

new Bucket("private", {
    acl: "private",
    vpcId: network.then((n) => n.vpcId),
    source: bucket.id,
});
