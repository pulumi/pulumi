import * as pulumi from "@pulumi/pulumi";

async function main() {
    await pulumi.runtime.checkPulumiVersion("=3.1.2")
}

main()
