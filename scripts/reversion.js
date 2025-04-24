// Copyright 2018, Pulumi Corporation.  All rights reserved.

var fs = require("fs");

if (process.argv.length < 4) {
    console.error("error: missing arguments; usage: <script> <file> <version>");
    process.exit(1);
}

var file = process.argv[2];
var data = fs.readFileSync(file).toString("utf8");
var version = process.argv[3];
if (version && version[0] === "v") {
    version = version.substring(1);
}

var versionPath = __dirname + "/../sdk/.version";
var versionToReplace = fs.readFileSync(versionPath).toString("utf8").trim();
var regexp = new RegExp(versionToReplace, "g");
fs.writeFileSync(file, data.replace(regexp, version));
