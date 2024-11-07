// This tests that invokes cannot depend on things which are not resources.

let pulumi = require("../../../../../");

const dependsOn = pulumi.output(Promise.resolve([Promise.resolve(1)]));
pulumi.runtime.invokeOutput("test:index:echo", {}, { dependsOn });
