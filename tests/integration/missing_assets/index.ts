import * as pulumi from "@pulumi/pulumi";
import { R } from "./res";

new R("missingAsset", {
    source: new pulumi.asset.FileAsset("MISSING"),
});
