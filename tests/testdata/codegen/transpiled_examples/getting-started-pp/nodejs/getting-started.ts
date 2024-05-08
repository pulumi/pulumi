import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const mybucket = new aws.s3.Bucket("mybucket", {website: {
    indexDocument: "index.html",
}});
const indexhtml = new aws.s3.BucketObject("indexhtml", {
    bucket: mybucket.id,
    source: new pulumi.asset.StringAsset("<h1>Hello, world!</h1>"),
    acl: "public-read",
    contentType: "text/html",
});
export const bucketEndpoint = pulumi.interpolate`http://${mybucket.websiteEndpoint}`;
