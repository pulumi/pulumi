import pulumi

def can_(fn):
    try:
        _result = fn()
        return True
    except:
        return False


config = pulumi.Config()
a_map = config.require_object("aMap")
pulumi.export("plainTrySuccess", can_(lambda: a_map["a"]))
pulumi.export("plainTryFailure", can_(lambda: a_map["b"]))
a_secret_map = pulumi.Output.secret(a_map)
pulumi.export("outputTrySuccess", can_(lambda: a_secret_map["a"]))
pulumi.export("outputTryFailure", can_(lambda: a_secret_map["b"]))
an_object = config.require_object("anObject")
pulumi.export("dynamicTrySuccess", can_(lambda: an_object["a"]))
pulumi.export("dynamicTryFailure", can_(lambda: an_object["b"]))
a_secret_object = pulumi.Output.secret(an_object)
pulumi.export("outputDynamicTrySuccess", can_(lambda: a_secret_object["a"]))
pulumi.export("outputDynamicTryFailure", can_(lambda: a_secret_object["b"]))
pulumi.export("plainTryNull", can_(lambda: an_object["opt"]))
pulumi.export("outputTryNull", can_(lambda: a_secret_object["opt"]))
