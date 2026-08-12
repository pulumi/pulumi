# Check that withV2 is generated against the v2 SDK and not against the V26 SDK,
# and that the version resource option is elided.
resource "withV2" "simple:index:Resource" {
    value = true
    options {
        version = "2.0.0"
    }
}
