# The schema provider covers interesting schema shapes.
resource "schemaprov" "pulumi:providers:testconfigprovider" {

    # Strings
    s1 = ""
    s2 = "x"
    # Test a JSON-like string to see if it trips up JSON detectors.
    s3 = "{}"

    # Integers
    i1 = 0
    i2 = 42

    # Numbers
    n1 = 0
    n2 = 42.42

    # Boolean values
    b1 = true
    b2 = false

    # Lists, `ls` is a list of strings, `li` is a list of integers.
    ls1 = []
    ls2 = ["", "foo"]
    li1 = [1, 2]

    # Maps, `ms` is a map of strings, `mi` is a map of integers.
    ms1 = {}
    ms2 = {
       key1 = "value1"
       key2 = "value2"
    }
    mi1 = {
       key1 = 0
       key2 = 42
    }

    # Objects; each object has just one field "x"; if `os` it is a string, `oi` is an integer and so on.
    os1 = {}
    os2 = {
       x = "x-value"
    }
    oi1 = {
       x = 42
    }
}

resource "schemaconf" "testconfigprovider:index:ConfigGetter" {
  options {
    provider = schemaprov
  }
}

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
