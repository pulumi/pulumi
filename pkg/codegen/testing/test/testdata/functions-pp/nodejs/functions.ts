import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as crypto from "crypto";
import * as fs from "fs";

function computeFilebase64sha256(path: string): string {
	const fileData = Buffer.from(fs.readFileSync(path, 'binary'))
	return crypto.createHash('sha256').update(fileData).digest('hex')
}

function mimeType(path: string): string {
    throw new Error("mimeType not implemented, use the mime or mime-types package instead");
}

const encoded = Buffer.from("haha business").toString("base64");
const decoded = Buffer.from(encoded, "base64").toString("utf8");
const joined = [
    encoded,
    decoded,
    "2",
].join("-");
// tests that we initialize "var, err" with ":=" first, then "=" subsequently (Go specific)
const zone = aws.getAvailabilityZones({});
const zone2 = aws.getAvailabilityZones({});
const bucket = new aws.s3.Bucket("bucket", {});
const encoded2 = bucket.id.apply(id => Buffer.from(id).toString("base64"));
const decoded2 = bucket.id.apply(id => Buffer.from(id, "base64").toString("utf8"));
const secretValue = pulumi.secret("hello");
const plainValue = pulumi.unsecret(secretValue);
const currentStack = pulumi.getStack();
const currentProject = pulumi.getProject();
const workingDirectory = process.cwd();
const fileMimeType = mimeType("./base64.txt");
// using the filebase64 function
const first = new aws.s3.BucketObject("first", {
    bucket: bucket.id,
    source: new pulumi.asset.StringAsset(fs.readFileSync("./base64.txt", { encoding: "base64" })),
    contentType: fileMimeType,
    tags: {
        stack: currentStack,
        project: currentProject,
        cwd: workingDirectory,
    },
});
// using the filebase64sha256 function
const second = new aws.s3.BucketObject("second", {
    bucket: bucket.id,
    source: new pulumi.asset.StringAsset(computeFilebase64sha256("./base64.txt")),
});
// using the sha1 function
const third = new aws.s3.BucketObject("third", {
    bucket: bucket.id,
    source: new pulumi.asset.StringAsset(crypto.createHash('sha1').update("content").digest('hex')),
});
