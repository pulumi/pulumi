import pulumi
import pulumi_testconfigprovider as testconfigprovider

# The schema provider covers interesting schema shapes.
schemaprov = testconfigprovider.Provider("schemaprov",
    s1="",
    s2="x",
    s3="{}",
    i1=0,
    i2=42,
    n1=0,
    n2=42.42,
    b1=True,
    b2=False,
    ls1=[],
    ls2=[
        "",
        "foo",
    ],
    li1=[
        1,
        2,
    ],
    ms1={},
    ms2={
        "key1": "value1",
        "key2": "value2",
    },
    mi1={
        "key1": 0,
        "key2": 42,
    },
    os1=testconfigprovider.Ts1Args(),
    os2=testconfigprovider.Ts2Args(
        x="x-value",
    ),
    oi1=testconfigprovider.Ti1Args(
        x=42,
    ))
schemaconf = testconfigprovider.ConfigGetter("schemaconf", opts = pulumi.ResourceOptions(provider=schemaprov))
# The program_secret_provider covers scenarios where user passes secret values to the provider.
programsecretprov = testconfigprovider.Provider("programsecretprov",
    s1=testconfigprovider.to_secret_output(s="SECRET").apply(lambda invoke: invoke.s),
    i1=testconfigprovider.to_secret_output(i=1234567890).apply(lambda invoke: invoke.i),
    n1=testconfigprovider.to_secret_output(n=123456.789).apply(lambda invoke: invoke.n),
    b1=testconfigprovider.to_secret_output(b=True).apply(lambda invoke: invoke.b),
    ls1=testconfigprovider.to_secret_output(ls=[
        "SECRET",
        "SECRET2",
    ]).apply(lambda invoke: invoke.ls),
    ls2=[
        "VALUE",
        testconfigprovider.to_secret_output(s="SECRET").apply(lambda invoke: invoke.s),
    ],
    ms2={
        "key1": "value1",
        "key2": testconfigprovider.to_secret_output(s="SECRET").apply(lambda invoke: invoke.s),
    },
    os2=testconfigprovider.Ts2Args(
        x=testconfigprovider.to_secret_output(s="SECRET").apply(lambda invoke: invoke.s),
    ))
programsecretconf = testconfigprovider.ConfigGetter("programsecretconf", opts = pulumi.ResourceOptions(provider=programsecretprov))
