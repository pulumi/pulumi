// Copyright 2026, Pulumi Corporation.

const fs = require("fs");
const path = require("path");

const sentinelDir = process.env.SENTINEL_DIR;
if (!sentinelDir) {
    throw new Error("SENTINEL_DIR not set");
}

fs.writeFileSync(path.join(sentinelDir, "started"), "ok");

process.on("SIGINT", () => { });

setInterval(() => { }, 60000);
