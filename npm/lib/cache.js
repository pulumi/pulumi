// Copyright 2026, Pulumi Corporation. All rights reserved.

"use strict";

const os = require("os");
const path = require("path");

// cacheDir returns the install root for a specific Pulumi version, mirroring
// the default path used by the Automation API (PulumiCommand.install). This
// lets both systems share an installation without re-downloading.
function cacheDir(version) {
    const home = process.env.PULUMI_HOME || path.join(os.homedir(), ".pulumi");
    return path.join(home, "versions", version);
}

module.exports = { cacheDir };
