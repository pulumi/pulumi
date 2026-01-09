import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const myBucket = new aws.s3.Bucket("myBucket", {website: {
    indexDocument: "index.html",
}});
const ownershipControls = new aws.s3.BucketOwnershipControls("ownershipControls", {
    bucket: myBucket.id,
    rule: {
        objectOwnership: "ObjectWriter",
    },
});
const publicAccessBlock = new aws.s3.BucketPublicAccessBlock("publicAccessBlock", {
    bucket: myBucket.id,
    blockPublicAcls: false,
});
const indexHtml = new aws.s3.BucketObject("index.html", {
    bucket: myBucket.id,
    source: new pulumi.asset.FileAsset("./index.html"),
    contentType: "text/html",
    acl: "public-read",
}, {
    dependsOn: [
        publicAccessBlock,
        ownershipControls,
    ],
});
export const bucketName = myBucket.id;
export const bucketEndpoint = pulumi.interpolate`http://${myBucket.websiteEndpoint}`;
