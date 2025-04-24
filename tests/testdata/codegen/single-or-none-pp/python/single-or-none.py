import pulumi

def single_or_none(elements):
    if len(elements) != 1:
        raise Exception("single_or_none expected input list to have a single element")
    return elements[0]


pulumi.export("result", single_or_none([1]))
