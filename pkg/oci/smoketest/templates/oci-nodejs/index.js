"use strict";
// A minimal OCI-mode Pulumi program. It runs as a container (runtime: oci), built
// from the Dockerfile in this directory. Add resources as usual; run
// `pulumi package add <provider>` to generate and wire a provider SDK.
const pulumi = require("@pulumi/pulumi");

exports.greeting = "hello from ${PROJECT}, an OCI nodejs program";
