import pulumi
from converted import Converted
import pulumi_primitive as primitive

direct = primitive.Resource("direct",
    boolean=True,
    float=3.14,
    integer=42,
    string="false",
    number_array=[
        float(-1),
        float(0),
        float(1),
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
        float(10),
        float(11),
    ], 
    'booleanMap': {
        "left": True,
        "right": False,
    }})
