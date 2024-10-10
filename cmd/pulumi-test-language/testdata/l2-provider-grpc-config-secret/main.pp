# The program_secret_provider covers scenarios where user passes secret values to the provider.
resource "programsecretprov" "pulumi:providers:testconfigprovider" {
    s1 = invoke("testconfigprovider:index:toSecret", {s = "SECRET"}).s
    i1 = invoke("testconfigprovider:index:toSecret", {i = 1234567890}).i
    n1 = invoke("testconfigprovider:index:toSecret", {n = 123456.7890}).n
    b1 = invoke("testconfigprovider:index:toSecret", {b = true}).b

    ls1 = invoke("testconfigprovider:index:toSecret", {ls = ["SECRET", "SECRET2"]}).ls
    ls2 = ["VALUE", invoke("testconfigprovider:index:toSecret", {s = "SECRET"}).s]

    # TODO[pulumi/pulumi#17535] this currently breaks Go compilation unfortunately.
    # ms1 = invoke("testconfigprovider:index:toSecret", {ms = { key1 = "SECRET", key2 = "SECRET2" }})

    ms2 = {
       key1 = "value1"
       key2 = invoke("testconfigprovider:index:toSecret", {s = "SECRET"}).s
    }

    # TODO[pulumi/pulumi#17535] this breaks Go compilation as well.
    # os1 = invoke("testconfigprovider:index:toSecret", {os = { x = "SECRET" }}).os

    os2 = { x = invoke("testconfigprovider:index:toSecret", {s = "SECRET"}).s }
}

resource "programsecretconf" "testconfigprovider:index:ConfigGetter" {
  options {
    provider = programsecretprov
  }
}
