import pulumi
import pulumi_nestedobject as nestedobject
import pulumi_simple as simple

target = simple.Resource("target", value=True)
other = nestedobject.Container("other", inputs=["a"])
skipped = nestedobject.Container("skipped", inputs=["b"])
pulumi.export("skippedOutput", skipped.details.apply(lambda details: f"skipped-{details[0].key}"))
