import * as pulumi from "@pulumi/pulumi";

export = async () => {
    const config = new pulumi.Config();
    const cidrBlock = config.get("cidrBlock") || "Test config variable";
    return {
        cidrBlock: cidrBlock,
    };
}
