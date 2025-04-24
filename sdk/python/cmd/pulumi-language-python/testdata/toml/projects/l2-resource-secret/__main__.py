import pulumi
import pulumi_secret as secret

res = secret.Resource("res",
    private="closed",
    public="open",
    private_data={
        "private": "closed",
        "public": "open",
    },
    public_data={
        "private": "closed",
        "public": "open",
    })
