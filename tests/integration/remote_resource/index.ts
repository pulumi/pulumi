import * as pulumi from "@pulumi/pulumi";
import * as path from "path";

const p = pulumi.runtime.invoke("pulumi:pulumi:remoteConstructResource", {
    runtime: "python",
    path: path.join(__dirname, "mypyapp"),
    monitorAddr: process.env["PULUMI_NODEJS_MONITOR"],
}, { async: true });

p.then(o => console.log(JSON.stringify(o)));
