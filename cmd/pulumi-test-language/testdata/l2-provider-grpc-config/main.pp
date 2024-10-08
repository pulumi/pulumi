resource "prov1" "pulumi:providers:testconfigprovider" {

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

resource "c1" "testconfigprovider:index:ConfigGetter" {
  options {
    provider = prov1
  }
}
