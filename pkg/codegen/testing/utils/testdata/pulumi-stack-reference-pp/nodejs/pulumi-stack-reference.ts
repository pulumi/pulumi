import * as pulumi from "@pulumi/pulumi";

const stackRef = new pulumi.StackReference("stackRef", {name: "foo/bar/dev"});
