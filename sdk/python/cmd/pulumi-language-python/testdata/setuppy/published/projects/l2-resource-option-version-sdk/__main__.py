import pulumi
import pulumi_simple as simple

# Check that withV2 is generated against the v2 SDK and not against the V26 SDK,
# and that the version resource option is elided.
with_v2 = simple.Resource("withV2", value=True)
