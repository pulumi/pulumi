import pulumi

def single_or_none(elements):
    if len(elements) == 1:
        return elements[0]
    return None


pulumi.export("result", single_or_none([1]))
