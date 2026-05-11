import * as pulumi from "@pulumi/pulumi";
import * as crypto from "crypto";

export = async () => {
    const config = new pulumi.Config();
    const input = config.require("input");
    const hash = crypto.createHash('sha1').update(input).digest('hex');
    return {
        hash: hash,
    };
}
