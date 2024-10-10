import pulumi
import pulumi_testconfigprovider as testconfigprovider

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
