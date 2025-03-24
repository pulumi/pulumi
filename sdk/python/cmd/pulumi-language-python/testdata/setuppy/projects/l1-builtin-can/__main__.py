import pulumi

def can_(fn):
    try:
        _result = fn()
        return True
    except:
        return False


str = "str"
config = pulumi.Config()
object = config.require_object("object")
another_object = {
    "nested": "nestedValue",
}
pulumi.export("canFalse", can_(lambda: object["a"]))
pulumi.export("canFalseDoubleNested", can_(lambda: object["a"]["b"]))
pulumi.export("canTrue", can_(lambda: another_object["nested"]))
