import pulumi

names = [
    "alpha",
    "beta",
    "gamma",
]
tags = {
    "Environment": "production",
    "Team": "infra",
}
pulumi.export("prefixed", [f"prefix-{n}" for n in names])
pulumi.export("filtered", [n for n in names if n != "beta"])
pulumi.export("indexed", [f"{i}:{n}" for i, n in enumerate(names)])
pulumi.export("tagList", [f"{k}={v}" for k, v in sorted(tags.items())])
pulumi.export("prefixedMap", {n: f"prefix-{n}" for n in names})
pulumi.export("filteredTags", {k: v for k, v in sorted(tags.items()) if k != "Team"})
