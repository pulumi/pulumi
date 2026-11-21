import * as pulumi from "@pulumi/pulumi";
import * as nested_failure from "@pulumi/nested-failure";

const outer = new nested_failure.Wrapper("outer", {});
