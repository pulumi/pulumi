import pulumi

data = [{
    "usingKey": entry["key"],
    "usingValue": entry["value"],
} for entry in [{"key": k, "value": v} for k, v in [
    1,
    2,
    3,
]]]
