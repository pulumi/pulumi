resource "res" "primitive:index:Resource" {
    boolean = true
    float = 3.14
    integer = 42
    string = "hello"
    numberArray = [-1.0, 0.0, 1.0]
    booleanMap = {
        "t": true,
        "f": false,
    }
}