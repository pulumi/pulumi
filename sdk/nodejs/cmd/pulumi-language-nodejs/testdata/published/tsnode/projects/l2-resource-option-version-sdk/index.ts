import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";

// Check that withV2 is generated against the v2 SDK and not against the V26 SDK,
// and that the version resource option is elided.
const withV2 = new simple.Resource("withV2", {value: true});
