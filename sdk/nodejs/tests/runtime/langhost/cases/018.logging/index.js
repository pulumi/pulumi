let pulumi = require("../../../../../");

pulumi.log.info("info message");
pulumi.log.warn("warning message");
pulumi.log.error("error message");

class FakeResource extends pulumi.CustomResource {
    constructor(name) {
        super("test:index:FakeResource", name);
    }
}

const res = new FakeResource("test");
pulumi.log.info("attached to resource", res);
pulumi.log.info("with streamid", res, 42);
