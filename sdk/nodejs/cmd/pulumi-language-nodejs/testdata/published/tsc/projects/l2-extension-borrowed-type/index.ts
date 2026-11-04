import * as pulumi from "@pulumi/pulumi";
import * as myborrow from "@pulumi/myborrow";

const widget = new myborrow.Widget("widget", {});
export const metadataName = widget.metadata.name;
