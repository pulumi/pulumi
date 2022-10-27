import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const bucket: aws.s3.Bucket[] = [];
for (const range = {value: 0}; range.value < 10; range.value++) {
    bucket.push(new aws.s3.Bucket(`bucket-${range.value}`, {website: {
        indexDocument: `index-${range.value}.html`,
    }}));
}
