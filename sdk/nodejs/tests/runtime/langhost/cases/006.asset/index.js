// This tests simple creation of assets.

let fabric = require("../../../../../");

class FileResource extends fabric.Resource {
    constructor(name, data) {
        super("test:index:FileResource", name, {
            "data": data,
        });
    }
}

new FileResource("file1", new fabric.asset.FileAsset("./testdata.txt"));

