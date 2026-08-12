// Copyright 2026, Pulumi Corporation.

"use strict";

const provider = require("@pulumi/pulumi/provider");
const fs = require("fs");
const path = require("path");

class MyProvider {
    constructor(version) {
        this.version = version;
        this.schema = JSON.stringify({
            name: "test-provider",
            version: "0.0.1",
            resources: {
                "test-provider:index:Component": {
                    isComponent: true,
                },
            },
        });
    }

    async construct(name, type, inputs, options) {
        const sentinelDir = process.env.SENTINEL_DIR || ".";

        // Write "started" sentinel to indicate construct has been entered.
        fs.writeFileSync(path.join(sentinelDir, "started"), "started");

        // Block forever
        return new Promise(() => {});
    }

    async cancel() {
        const sentinelDir = process.env.SENTINEL_DIR || ".";
        fs.writeFileSync(path.join(sentinelDir, "graceful-shutdown"), "graceful-shutdown");
    }
}

const prov = new MyProvider("0.0.1");
provider.main(prov, process.argv.slice(2));
