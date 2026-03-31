resource "unsetA" "optionalprimitive:index:Resource" {}

resource "unsetB" "optionalprimitive:index:Resource" {
    boolean = unsetA.boolean
    float = unsetA.float
    integer = unsetA.integer
    string = unsetA.string
    numberArray = unsetA.numberArray
    booleanMap = unsetA.booleanMap
}

output "unsetBoolean" {
    value = unsetB.boolean == null ? "null" : "not null"
}

output "unsetFloat" {
    value = unsetB.float == null ? "null" : "not null"
}

output "unsetInteger" {
    value = unsetB.integer == null ? "null" : "not null"
}

output "unsetString" {
    value = unsetB.string == null ? "null" : "not null"
}

output "unsetNumberArray" {
    value = unsetB.numberArray == null ? "null" : "not null"
}

output "unsetBooleanMap" {
    value = unsetB.booleanMap == null ? "null" : "not null"
}

resource "setA" "optionalprimitive:index:Resource" {
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

resource "setB" "optionalprimitive:index:Resource" {
    boolean = setA.boolean
    float = setA.float
    integer = setA.integer
    string = setA.string
    numberArray = setA.numberArray
    booleanMap = setA.booleanMap
}

resource "sourcePrimitive" "primitive:index:Resource" {
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

resource "fromPrimitive" "optionalprimitive:index:Resource" {
    boolean = sourcePrimitive.boolean
    float = sourcePrimitive.float
    integer = sourcePrimitive.integer
    string = sourcePrimitive.string
    numberArray = sourcePrimitive.numberArray
    booleanMap = sourcePrimitive.booleanMap
}

output "setBoolean" {
    value = setB.boolean
}

output "setFloat" {
    value = setB.float
}

output "setInteger" {
    value = setB.integer
}

output "setString" {
    value = setB.string
}

output "setNumberArray" {
    value = setB.numberArray
}

output "setBooleanMap" {
    value = setB.booleanMap
}
