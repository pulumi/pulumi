// This tests the ability to use promises for resource properties.

let pulumi = require("../../../../../");
let fs = require("fs");

class FileResource extends pulumi.CustomResource {
    constructor(name, data) {
        super("test:index:FileResource", name, {
            data: data,
        });
    }
}

new FileResource(
    "file1",
    new Promise((resolve, reject) => {
        fs.readFile("./testdata.txt", (err, data) => {
            if (err) {
                reject(err);
            } else {
                resolve(data.toString("utf-8"));
            }
        });
    }),
);
