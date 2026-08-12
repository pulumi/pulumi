resource "example1" "discriminated-union:index:Example" {
  unionOf = { discriminantKind = "variant1", field1 = "v1 union" }
  arrayOfUnionOf = [{ discriminantKind = "variant1", field1 = "v1 array(union)" }]
}

resource "example2" "discriminated-union:index:Example" {
  unionOf = { discriminantKind = "variant2", field2 = "v2 union" }
  arrayOfUnionOf = [{ discriminantKind = "variant2", field2 = "v2 array(union)" }, { discriminantKind = "variant1", field1 = "v1 array(union)" }]
}
