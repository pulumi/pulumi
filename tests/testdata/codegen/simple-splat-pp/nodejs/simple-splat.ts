import * as pulumi from "@pulumi/pulumi";
import * as splat from "@pulumi/splat";

const allKeys = splat.getSshKeys({});
const main = new splat.Server("main", {sshKeys: allKeys.then(allKeys => allKeys.sshKeys.map(__item => __item.name))});
