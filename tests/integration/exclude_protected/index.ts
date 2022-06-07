import * as pulumi from "@pulumi/pulumi";


class Resource extends pulumi.ComponentResource {
    constructor(name: string, _?: {}, opts?: pulumi.ComponentResourceOptions) {
        super("my:module:Resource", name, {}, opts);
    }
}



const bucket1 = new Resource("my-bucket", {}, { protect: true });
// Because `protect` is explicitly set to false, we will delete this.
new Resource("my-bucket-child", {}, { protect: false, parent: bucket1 });
new Resource("my-bucket-child-protected", {}, { protect: true, parent: bucket1 });

const bucket2 = new Resource("my-2bucket", {}, { protect: false });
new Resource("my-2bucket-child", {}, { protect: false, parent: bucket2 });
new Resource("my-2bucket-protected-child", {}, { protect: true, parent: bucket2 });

const p = new Resource("provided-bucket", {}, { protect: true })
// Inherits protected status from `p`. This is protected in the state, and is thus safe.
new Resource("provided-bucket-child", {}, { parent: p })
new Resource("provided-bucket-child-unprotected", {}, { parent: p, protect: false })

// If possible, we should do a test with providers, that looks something like
// this. Doing a provider test with component resources is problematic because
// `ComponentResources` don't have CRUD operations.
//
// import * as aws from "@pulumi/aws";
// new aws.s3.Bucket("provider-unprotected", {}, { provider: prov })
// const p = new aws.s3.Bucket("provided-bucket", {}, { provider: prov, protect: true })
// // Inherits protected status from `p`. This is protected in the state, and is thus safe.
// new aws.s3.Bucket("provided-bucket-child", {}, { parent: p })
// new aws.s3.Bucket("provided-bucket-child-unprotected", {}, { parent: p, protect: false })
