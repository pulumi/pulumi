import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const encoded = Buffer.from("haha business").toString("base64");
const decoded = Buffer.from(encoded, "base64").toString("utf8");
const joined = [
    encoded,
    decoded,
    "2",
].join("-");
const zone = aws.getAvailabilityZones({});
const zone2 = aws.getAvailabilityZones({});
const bucket = new aws.s3.Bucket("bucket", {});
const encoded2 = bucket.id.apply(id => Buffer.from(id).toString("base64"));
const decoded2 = bucket.id.apply(id => Buffer.from(id, "base64").toString("utf8"));
