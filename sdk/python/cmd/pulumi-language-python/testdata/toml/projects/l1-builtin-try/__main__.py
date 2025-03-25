import pulumi
import typing

def tryOutput_(*fns) -> pulumi.Output[typing.Any]:
	if len(fns) == 0:
		raise Exception("tryOutput: all parameters failed")

	fn, *rest = fns
	try:
		result = fn()
		return pulumi.Output.from_input(result).apply(lambda result: result if result != pulumi.UNDEFINED else tryOutput_(*rest))
	except:
		return tryOutput_(*rest)


def try_(*fns) -> typing.Any:
		for fn in fns:
			try:
				return fn()
			except:
				continue
		raise Exception("try: all parameters failed")
	

config = pulumi.Config()
a_map = config.require_object("aMap")
pulumi.export("plainTrySuccess", try_(
    lambda: a_map["a"],
    lambda: "fallback"
))
pulumi.export("plainTryFailure", try_(
    lambda: a_map["b"],
    lambda: "fallback"
))
a_secret_map = pulumi.Output.secret(a_map)
pulumi.export("outputTrySuccess", tryOutput_(
    lambda: a_secret_map["a"],
    lambda: "fallback"
))
pulumi.export("outputTryFailure", tryOutput_(
    lambda: a_secret_map["b"],
    lambda: "fallback"
))
an_object = config.require_object("anObject")
pulumi.export("dynamicTrySuccess", tryOutput_(
    lambda: an_object["a"],
    lambda: "fallback"
))
pulumi.export("dynamicTryFailure", tryOutput_(
    lambda: an_object["b"],
    lambda: "fallback"
))
a_secret_object = pulumi.Output.secret(an_object)
pulumi.export("outputDynamicTrySuccess", tryOutput_(
    lambda: a_secret_object["a"],
    lambda: "fallback"
))
pulumi.export("outputDynamicTryFailure", tryOutput_(
    lambda: a_secret_object["b"],
    lambda: "fallback"
))
pulumi.export("plainTryNull", tryOutput_(
    lambda: an_object["opt"],
    lambda: "fallback"
))
pulumi.export("outputTryNull", tryOutput_(
    lambda: a_secret_object["opt"],
    lambda: "fallback"
))
