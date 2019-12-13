import * as pulumi from "@pulumi/pulumi";
import * as path from "path";

function createResource(runtime: string, dir: string) {
    const p = pulumi.runtime.invoke("pulumi:pulumi:remoteConstructResource", {
        runtime: runtime,
        pwd: path.join(__dirname, dir),
        path: "",
        monitorAddr: process.env["PULUMI_NODEJS_MONITOR"],
        dryRun: pulumi.runtime.isDryRun(),
        project: pulumi.getProject(),
        stack: "remote",
    }, { async: true });

    p.then(o => console.log(JSON.stringify(o)));
}

createResource("nodejs", "myjscomponent");
createResource("python", "mypyapp");
