resource "direct" "primitive:index:Resource" {
    boolean = "true"
    float = "3.14"
    integer = "42"
    string = false
    numberArray = ["-1", "0", "1"]
    booleanMap = {
        "t": "true",
        "f": "false",
    }
}

component "converted" "./converted" {
    boolean = "false"
    float = "2.5"
    integer = "7"
    string = true
    numberArray = ["10", "11"]
    booleanMap = {
        "left": "true",
        "right": "false",
    }
}
