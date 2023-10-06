import pulumi
import json
import pulumi_aws as aws
import pulumi_random as random

data = [
    "bob",
    "john",
    "carl",
]
user = []
for range in [{"key": k, "value": v} for [k, v] in enumerate(data)]:
    user.append(random.RandomPassword(f"user-{range['key']}", length=16))
db_users = []
for range in [{"key": k, "value": v} for [k, v] in enumerate(data)]:
    db_users.append(aws.secretsmanager.SecretVersion(f"dbUsers-{range['key']}",
        secret_id="mySecret",
        secret_string=user[range["value"]].result.apply(lambda result: json.dumps({
            "username": range["value"],
            "password": result,
        }))))
