import pulumi
import pulumi_output as output
import pulumi_simple as simple

# Produces outputs that are unknown during preview via the output provider.
src = output.Resource("src", value=float(1))
# value is an unknown bool during preview, known-true on update. The policy
# pack should "skip" (report not-applicable) for this resource during preview.
unknown_res = simple.Resource("unknownRes", value=src.output.apply(lambda output: output == "hello"))
# value is a secret-wrapped unknown bool during preview, secret-wrapped true on
# update. The policy pack should also "skip" during preview, and must be able
# to unwrap the secret and inspect it on update.
secret_unknown_res = simple.Resource("secretUnknownRes", value=src.secret_output.apply(lambda secret_output: secret_output == "hello"))
# value is a known plain bool. The policy pack inspects it directly.
known_res = simple.Resource("knownRes", value=False)
# value is a known secret-wrapped bool. The policy pack must unwrap the secret
# to inspect the underlying value.
secret_known_res = simple.Resource("secretKnownRes", value=pulumi.Output.secret(False))
