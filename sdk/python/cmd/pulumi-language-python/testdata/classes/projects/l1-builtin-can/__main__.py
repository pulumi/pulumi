import pulumi

def canOutput_(fn) -> pulumi.Output[bool]:
	try:
		result = pulumi.Output.from_input(fn())
		return result.apply(lambda x: x != pulumi.UNDEFINED)
	except:
		return pulumi.Output.from_input(False)


def can_(fn) -> bool:
		try:
			result = fn()
			return True
		except:
			return False
	

config = pulumi.Config()
a_map = config.require_object("aMap")
pulumi.export("plainCanSuccess", can_(lambda: a_map["a"]))
pulumi.export("plainCanFailure", can_(lambda: a_map["b"]))
a_secret_map = pulumi.Output.secret(a_map)
pulumi.export("outputCanSuccess", canOutput_(lambda: a_secret_map["a"]))
pulumi.export("outputCanFailure", canOutput_(lambda: a_secret_map["b"]))
an_object = config.require_object("anObject")
pulumi.export("dynamicCanSuccess", canOutput_(lambda: an_object["a"]))
pulumi.export("dynamicCanFailure", canOutput_(lambda: an_object["b"]))
a_secret_object = pulumi.Output.secret(an_object)
pulumi.export("outputDynamicCanSuccess", canOutput_(lambda: a_secret_object["a"]))
pulumi.export("outputDynamicCanFailure", canOutput_(lambda: a_secret_object["b"]))
