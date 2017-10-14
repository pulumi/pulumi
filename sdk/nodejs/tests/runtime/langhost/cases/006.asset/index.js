// This tests simple creation of assets.

let pulumi = require("../../../../../");

class FileResource extends pulumi.ExternalResource {
    constructor(name, data) {
        super("test:index:FileResource", name, {
            "data": data,
        });
    }
}

new FileResource("file1", new pulumi.asset.FileAsset("./testdata.txt"));

