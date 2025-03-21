import pulumi
import typing

def tryOutput_(*fns) -> pulumi.Output[typing.Any]:
	if len(fns) == 0:
		raise Exception("tryOutput: all parameters failed")

	fn, *rest = fns
	result_output = None
	try:
		result = fn()
		result_output = pulumi.Output.from_input(result)
	except:
		return tryOutput_(*rest)

	return result_output


def try_(*fns) -> typing.Any:
		for fn in fns:
			try:
				return fn()
			except:
				continue
		raise Exception("try: all parameters failed")
	

str = "str"
a_list = [
    "a",
    "b",
    "c",
]
pulumi.export("nonOutputTry", try_(
    lambda: a_list[0],
    lambda: "fallback"
))
config = pulumi.Config()
object = config.require_object("object")
pulumi.export("trySucceed", tryOutput_(
    lambda: str,
    lambda: object["a"],
    lambda: "fallback"
))
pulumi.export("tryFallback1", tryOutput_(
    lambda: object["a"],
    lambda: "fallback"
))
pulumi.export("tryFallback2", tryOutput_(
    lambda: object["a"],
    lambda: object["b"],
    lambda: "fallback"
))
pulumi.export("tryMultipleTypes", tryOutput_(
    lambda: object["a"],
    lambda: object["b"],
    lambda: 42,
    lambda: "fallback"
))
