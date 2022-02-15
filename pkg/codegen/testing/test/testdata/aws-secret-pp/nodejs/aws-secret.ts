import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const dbCluster = new aws.rds.Cluster("dbCluster", {masterPassword: pulumi.secret("foobar")});
