import pulumi

def canOutput_(fn) -> pulumi.Output[bool]:
	try:
		result = pulumi.Output.from_input(fn())
		return result.apply(lambda x: x != None)
	except:
		return pulumi.Output.from_input(False)


def can_(fn) -> bool:
		try:
			result = fn()
			return True
		except:
			return False
	

str = "str"
a_list = [
    "a",
    "b",
    "c",
]
pulumi.export("nonOutputCan", can_(lambda: a_list[0]))
config = pulumi.Config()
object = config.require_object("object")
another_object = {
    "nested": "nestedValue",
}
pulumi.export("canFalse", canOutput_(lambda: object["a"]))
pulumi.export("canFalseDoubleNested", canOutput_(lambda: object["a"]["b"]))
pulumi.export("canTrue", can_(lambda: another_object["nested"]))
# canOutput should also generate, secrets are l1 functions which return outputs.
some_secret = pulumi.Output.secret({
    "a": "a",
})
pulumi.export("canOutput", canOutput_(lambda: some_secret).apply(lambda can: "true" if can else "false"))
