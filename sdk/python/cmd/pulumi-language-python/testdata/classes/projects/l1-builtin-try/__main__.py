import pulumi

def try_(*fns):
    for fn in fns:
        try:
            result = fn()
            return result
        except:
            continue
    return None


str = "str"
config = pulumi.Config()
object = config.require_object("object")
pulumi.export("trySucceed", try_(
    lambda: str,
    lambda: object["a"],
    lambda: "fallback"
))
pulumi.export("tryFallback1", try_(
    lambda: object["a"],
    lambda: "fallback"
))
pulumi.export("tryFallback2", try_(
    lambda: object["a"],
    lambda: object["b"],
    lambda: "fallback"
))
pulumi.export("tryMultipleTypes", try_(
    lambda: object["a"],
    lambda: object["b"],
    lambda: 42,
    lambda: "fallback"
))
