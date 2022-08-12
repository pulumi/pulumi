import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as aws_s3_static_website_bucket from "./modules/aws-s3-static-website-bucket";

export class Index extends pulumi.ComponentResource {
    public readonly s3Bucket: aws.s3.Bucket;
    public readonly childModule: aws_s3_static_website_bucket.Index;

    constructor(name: string, args: IndexArgs, opts: pulumi.ComponentResourceOptions = {}) {
        super("pkg:index:component", name, args, opts);

        // Local Terraform Child Module
        this.s3Bucket = new aws.s3.Bucket("s3Bucket", {bucket: "s3BucketId"}, {
            parent: this,
        });
        this.childModule = new aws_s3_static_website_bucket.Index("childModule", {customModuleParameter: this.s3Bucket.id}, {
            parent: this,
        });
    }
}

export interface IndexArgs {
}
