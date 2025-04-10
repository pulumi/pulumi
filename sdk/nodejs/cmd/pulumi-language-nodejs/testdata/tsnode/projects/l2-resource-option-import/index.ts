import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

const _import = new simple.Resource("import", {value: true}, {
    "import": "fakeID123",
});
const notImport = new simple.Resource("notImport", {value: true});
