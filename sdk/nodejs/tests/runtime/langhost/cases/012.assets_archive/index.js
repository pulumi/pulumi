// This tests the basic creation of a single resource with an assets archive property.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        let archive = new pulumi.asset.AssetArchive({
            "asset": new pulumi.asset.StringAsset("foo"),
            "archive": new pulumi.asset.AssetArchive({}),
        });
        super("test:index:MyResource", name, { "archive": archive });
    }
}

new MyResource("testResource1");
