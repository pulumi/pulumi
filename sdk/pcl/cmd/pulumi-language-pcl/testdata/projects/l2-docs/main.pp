resource "enumRes" "enum:index:Res" {
    intEnum = 1
    stringEnum = "one"
}

resource "res" "docs:index:Resource" {
    in = invoke("docs:index:fun", { in: false }).out
    externalEnum = "one"
}
