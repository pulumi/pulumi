import pulumi
from typing import Any
import json
import pulumi_aws as aws
import pulumi_random as random

data = [
    "bob",
    "john",
    "carl",
]
user: list[random.RandomPassword] = []
for user_range in [{"key": k, "value": v} for [k, v] in enumerate(data)]:
    user.append(random.RandomPassword(f"user-{user_range['key']}", length=16))
db_users: list[aws.secretsmanager.SecretVersion] = []
for db_users_range in [{"key": k, "value": v} for [k, v] in enumerate(data)]:
    db_users.append(aws.secretsmanager.SecretVersion(f"dbUsers-{db_users_range['key']}",
        secret_id="mySecret",
        secret_string=pulumi.Output.json_dumps({
            "username": db_users_range["value"],
            "password": user[db_users_range["value"]].result,
        })))
