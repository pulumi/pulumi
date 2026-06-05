resource "setRes" "optional-primitive-ref:index:Resource" {
    data = {
        boolean = true
        float = 3.14
        integer = 42
        string = "hello"
        numberArray = [-1.0, 0.0, 1.0]
        booleanMap = {
            "t" = true,
            "f" = false,
        }
    }
}

resource "unsetRes" "optional-primitive-ref:index:Resource" {
    data = {}
}

# Traversal through an output object (data) to an optional inner scalar.
# In Go this lowers to `setRes.Data.ApplyT(func(d Data) (*T, error) { ... return ?d.Field, nil })`
# where the inner field type is already a pointer - the SDK generator must not double-pointer it.
output "setBoolean" {
    value = setRes.data.boolean
}

output "setFloat" {
    value = setRes.data.float
}

output "setInteger" {
    value = setRes.data.integer
}

output "setString" {
    value = setRes.data.string
}

output "setNumberArray" {
    value = setRes.data.numberArray
}

output "setBooleanMap" {
    value = setRes.data.booleanMap
}

output "unsetBoolean" {
    value = unsetRes.data.boolean == null ? "null" : "not null"
}

output "unsetFloat" {
    value = unsetRes.data.float == null ? "null" : "not null"
}

output "unsetInteger" {
    value = unsetRes.data.integer == null ? "null" : "not null"
}

output "unsetString" {
    value = unsetRes.data.string == null ? "null" : "not null"
}

output "unsetNumberArray" {
    value = unsetRes.data.numberArray == null ? "null" : "not null"
}

output "unsetBooleanMap" {
    value = unsetRes.data.booleanMap == null ? "null" : "not null"
}
