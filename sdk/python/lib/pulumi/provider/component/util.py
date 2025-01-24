def camel_case(name: str) -> str:
    # TODO: might need to handle more cases here,
    # see https://github.com/pulumi/pulumi/blob/master/pkg/codegen/python/python.go#L49
    arr = name.split("_")
    return arr[0] + "".join([w.capitalize() for w in arr[1:]])


def python_name(name: str) -> str:
    result = ""
    for i, c in enumerate(name):
        if i > 0 and c.isupper():
            result += "_"
        result += c.lower()
    return result
