let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name, version) {
        super("test:index:MyResource", name, {}, { version: version });
    }
}

new MyResource("testResource", "0.19.1");
new MyResource("testResource2", "0.19.2");
new MyResource("testResource3");


pulumi.runtime.invoke("invoke:index:doit", {}, { version: "0.19.1" });
pulumi.runtime.invoke("invoke:index:doit_v2", {}, { version: "0.19.2" });
pulumi.runtime.invoke("invoke:index:doit_noversion", {});
