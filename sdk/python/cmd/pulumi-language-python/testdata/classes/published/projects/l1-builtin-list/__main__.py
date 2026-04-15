import pulumi

def single_or_none(elements):
    if len(elements) > 1:
        raise Exception("single_or_none expected input list to have a single element")
    return elements[0] if elements else None


config = pulumi.Config()
a_list = config.require_object("aList")
single_or_none_list = config.require_object("singleOrNoneList")
a_string = config.require("aString")
pulumi.export("elementOutput1", a_list[1])
pulumi.export("elementOutput2", a_list[2])
pulumi.export("joinOutput", "|".join(a_list))
pulumi.export("lengthOutput", len(a_list))
pulumi.export("splitOutput", a_string.split("-"))
pulumi.export("singleOrNoneOutput", [single_or_none(single_or_none_list)])
