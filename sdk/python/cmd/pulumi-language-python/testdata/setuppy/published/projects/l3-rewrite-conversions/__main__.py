import pulumi
from converted import Converted
import pulumi_primitive as primitive

direct = primitive.Resource("direct",
    boolean=True,
    float=3.14,
    integer=42,
    string="false",
    number_array=[
        -1,
        0,
        1,
    ],
    boolean_map={
        "t": True,
        "f": False,
    })
converted = Converted("converted", {
    'boolean': False, 
    'float': 2.5, 
    'integer': 7, 
    'string': "true", 
    'numberArray': [
        10,
        11,
    ], 
    'booleanMap': {
        "left": True,
        "right": False,
    }})
