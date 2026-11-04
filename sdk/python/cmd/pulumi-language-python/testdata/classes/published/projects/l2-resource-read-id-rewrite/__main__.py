import pulumi
import pulumi_read_id_rewrite as read_id_rewrite
import pulumi_simple as simple

res = read_id_rewrite.Resource.get("res", "existing-id", lookup="existing-key")
src = simple.Resource("src", value=True)
unknown_res = read_id_rewrite.Resource.get("unknownRes", src.id, lookup="another-key")
pulumi.export("resourceId", res.id)
pulumi.export("lookup", res.lookup)
pulumi.export("unknownResourceId", unknown_res.id)
pulumi.export("unknownLookup", unknown_res.lookup)
