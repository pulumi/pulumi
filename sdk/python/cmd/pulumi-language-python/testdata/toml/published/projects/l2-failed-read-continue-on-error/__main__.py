import pulumi
import pulumi_read as read

failing = read.Resource.get("failing", "existing-id", lookup="fail")
pulumi.export("failingValue", failing.value)
pulumi.export("failingLookup", failing.lookup)
