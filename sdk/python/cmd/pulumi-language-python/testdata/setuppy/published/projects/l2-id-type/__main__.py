import pulumi
import pulumi_primitive as primitive

# Test that the ID type is treated the same as a string type, despite being tracked as a distinct type. This 
# includes directly passing it to string fields, but also for bool and numeric values being able to cast to it.
source1 = primitive.Resource("source1",
    boolean=False,
    float=float(1),
    integer=2,
    string="1234",
    number_array=[float(3)],
    boolean_map={
        "source": False,
    })
source2 = primitive.Resource("source2",
    boolean=False,
    float=float(1),
    integer=2,
    string="true",
    number_array=[float(3)],
    boolean_map={
        "source": False,
    })
id_map = {
    "source1Token": source1.id,
    "source2Token": source2.id,
}
sink1 = primitive.Resource("sink1",
    boolean=False,
    float=id_map["source1Token"].apply(lambda x: float(x)),
    integer=id_map["source1Token"].apply(lambda x: int(x)),
    string=id_map["source1Token"],
    number_array=[id_map["source1Token"].apply(lambda x: float(x))],
    boolean_map={
        "sink": False,
    })
sink2 = primitive.Resource("sink2",
    boolean=id_map["source2Token"].apply(lambda x: x == "true"),
    float=float(1),
    integer=2,
    string="abc",
    number_array=[float(3)],
    boolean_map={
        "sink": id_map["source2Token"].apply(lambda x: x == "true"),
    })
pulumi.export("ids", id_map)
