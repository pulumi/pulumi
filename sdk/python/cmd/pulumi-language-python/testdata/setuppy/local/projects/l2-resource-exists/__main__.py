import pulumi
import pulumi_exists as exists

res = exists.Resource("res", value=True)
exists_result = exists.Resource.exists(res.id)
exists_with_state = exists.Resource.exists(res.id, value=True)
missing_with_state = exists.Resource.exists(res.id, value=False)
pulumi.export("existsResult", exists_result)
pulumi.export("existsWithState", exists_with_state)
pulumi.export("missingWithState", missing_with_state)
