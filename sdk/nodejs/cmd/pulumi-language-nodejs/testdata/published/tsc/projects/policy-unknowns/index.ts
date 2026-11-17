import * as pulumi from "@pulumi/pulumi";
import * as output from "@pulumi/output";
import * as simple from "@pulumi/simple";

// Produces outputs that are unknown during preview via the output provider.
const src = new output.Resource("src", {value: 1});
// value is an unknown bool during preview, known-true on update. The policy
// pack should "skip" (report not-applicable) for this resource during preview.
const unknownRes = new simple.Resource("unknownRes", {value: src.output.apply(output => output == "hello")});
// value is a secret-wrapped unknown bool during preview, secret-wrapped true on
// update. The policy pack should also "skip" during preview, and must be able
// to unwrap the secret and inspect it on update.
const secretUnknownRes = new simple.Resource("secretUnknownRes", {value: src.secretOutput.apply(secretOutput => secretOutput == "hello")});
// value is a known plain bool. The policy pack inspects it directly.
const knownRes = new simple.Resource("knownRes", {value: false});
// value is a known secret-wrapped bool. The policy pack must unwrap the secret
// to inspect the underlying value.
const secretKnownRes = new simple.Resource("secretKnownRes", {value: pulumi.secret(false)});
