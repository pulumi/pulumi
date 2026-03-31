resource "resExplicit" "primitive-defaults:index:Resource" {
    boolean = true
    float = 3.14
    integer = 42
    string = "hello"
}

resource "resDefaulted" "primitive-defaults:index:Resource" {
}
