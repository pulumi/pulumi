config booleanMap "map(bool)" {}

resource "res" "primitive:index:Resource" {
    boolean = false
    float = 2.17
    integer = -12
    string = "adversarial"
    numberArray = [0, 1]
    booleanMap = booleanMap
}

output booleanMap {
    value = res.booleanMap
}
