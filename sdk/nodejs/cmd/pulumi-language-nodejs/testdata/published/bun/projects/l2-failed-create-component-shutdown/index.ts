import * as pulumi from "@pulumi/pulumi";
import * as failingcomponent from "@pulumi/failingcomponent";

const component = new failingcomponent.Component("component", {});
