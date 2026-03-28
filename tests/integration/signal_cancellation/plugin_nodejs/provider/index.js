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

        process.on("SIGINT", () => {
            // Write "graceful-shutdown" sentinel to indicate we received the signal.
            fs.writeFileSync(path.join(sentinelDir, "graceful-shutdown"), "graceful-shutdown");
            process.exit(0);
        });

        // Block forever by returning a promise that never resolves. We exit in the sigint handler.
        return new Promise(() => { });
    }
}

const prov = new MyProvider("0.0.1");
provider.main(prov, process.argv.slice(2));
