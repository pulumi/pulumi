import pulumi

config = pulumi.Config()
a_map = config.require_object("aMap")
pulumi.export("entriesOutput", [{"key": k, "value": v} for k, v in sorted(a_map.items())])
pulumi.export("lookupOutput", a_map.get("keyPresent", "default"))
pulumi.export("lookupOutputDefault", a_map.get("keyMissing", "default"))
alternative_names = config.get_object("alternativeNames")
if alternative_names is None:
    alternative_names = {}
pulumi.export("names", [entry["value"] for entry in [{"key": k, "value": v} for k, v in sorted(alternative_names.items())]])
