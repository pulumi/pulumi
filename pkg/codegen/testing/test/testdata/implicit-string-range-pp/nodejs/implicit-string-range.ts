import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const config = new pulumi.Config();
// Number of AZs to cover in a given region
const azCount = config.get("azCount") || "10";
const bucketsPerAvailabilityZone: aws.s3.Bucket[] = [];
for (const range = {value: 0}; range.value < parseInt(azCount, 10); range.value++) {
    bucketsPerAvailabilityZone.push(new aws.s3.Bucket(`bucketsPerAvailabilityZone-${range.value}`, {website: {
        indexDocument: "index.html",
    }}));
}
