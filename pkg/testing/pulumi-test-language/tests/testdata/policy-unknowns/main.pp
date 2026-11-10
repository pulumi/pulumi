# Produces outputs that are unknown during preview via the output provider.
resource "src" "output:index:Resource" {
    value = 1
}

# value is an unknown bool during preview, known-true on update. The policy
# pack should "skip" (report not-applicable) for this resource during preview.
resource "unknownRes" "simple:index:Resource" {
    value = src.output == "hello"
}

# value is a secret-wrapped unknown bool during preview, secret-wrapped true on
# update. The policy pack should also "skip" during preview, and must be able
# to unwrap the secret and inspect it on update.
resource "secretUnknownRes" "simple:index:Resource" {
    value = src.secretOutput == "hello"
}

# value is a known plain bool. The policy pack inspects it directly.
resource "knownRes" "simple:index:Resource" {
    value = false
}

# value is a known secret-wrapped bool. The policy pack must unwrap the secret
# to inspect the underlying value.
resource "secretKnownRes" "simple:index:Resource" {
    value = secret(false)
}
