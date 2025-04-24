import * as pulumi from "@pulumi/pulumi";
import * as using_dashes from "@pulumi/using-dashes";

const main = new using_dashes.Dash("main", {stack: "dev"});
