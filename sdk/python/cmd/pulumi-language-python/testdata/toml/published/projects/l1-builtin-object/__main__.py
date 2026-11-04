import pulumi

config = pulumi.Config()
a_map = config.require_object("aMap")
pulumi.export("entriesOutput", [{"key": k, "value": v} for k, v in sorted(a_map.items())])
pulumi.export("lookupOutput", a_map.get("keyPresent", "default"))
pulumi.export("lookupOutputDefault", a_map.get("keyMissing", "default"))
pulumi.export("lookupLiteral", {
    "a": 1,
    "b": 2,
}.get("c", 3))
pulumi.export("lookupLiteralMismatchedType", {
    "a": 1,
    "b": 2,
}.get("c", True))
pulumi.export("deprecatedTwoArgumentLookup", {
    "a": 1,
    "b": 2,
}["a"])
alternative_names = config.get_object("alternativeNames")
if alternative_names is None:
    alternative_names = {}
pulumi.export("names", [entry["value"] for entry in [{"key": k, "value": v} for k, v in sorted(alternative_names.items())]])
pulumi.export("lengthOutput", len(a_map))
pulumi.export("lengthDynamicOutput", len(alternative_names))
