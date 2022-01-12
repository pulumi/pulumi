"use strict";
var childProcess = require("child_process");

var args = process.argv.slice(2);
var res = childProcess.spawnSync("pulumi", ["plugin", "install"].concat(args), {
    stdio: ["ignore", "inherit", "inherit"]
});

if (res.error && res.error.code === "ENOENT") {
    console.error("\nThere was an error installing the resource provider plugin. " +
            "It looks like `pulumi` is not installed on your system. " +
            "Please visit https://pulumi.com/ to install the Pulumi CLI.\n" +
            "You may try manually installing the plugin by running " +
            "`pulumi plugin install " + args.join(" ") + "`");
} else if (res.error || res.status !== 0) {
    console.error("\nThere was an error installing the resource provider plugin. " +
            "You may try to manually installing the plugin by running " +
            "`pulumi plugin install " + args.join(" ") + "`");
}

process.exit(0);
