// This tests the basic creation of a single resource with an assets archive property.

let pulumi = require("../../../../../");

class MyResource extends pulumi.CustomResource {
    constructor(name) {
        let archive = new pulumi.asset.AssetArchive({
            asset: new pulumi.asset.StringAsset("foo"),
            archive: new pulumi.asset.AssetArchive({}),
        });
        let archiveP = Promise.resolve(
            new pulumi.asset.AssetArchive({
                foo: new pulumi.asset.StringAsset("bar"),
            }),
        );
        let assetP = Promise.resolve(new pulumi.asset.StringAsset("baz"));
        super("test:index:MyResource", name, {
            archive: archive,
            archiveP: archiveP,
            assetP: assetP,
        });
    }
}

new MyResource("testResource1");
