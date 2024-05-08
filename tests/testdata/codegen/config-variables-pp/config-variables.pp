config "requiredString" "string" { }
config "requiredInt" "int" { }
config "requiredFloat" "number" { }
config "requiredBool" "bool" { }
config "requiredAny" "any" { }

config "optionalString" "string" { default = "defaultStringValue" }
config "optionalInt" "int" { default = 42 }
config "optionalFloat" "number" { default = 3.14 }
config "optionalBool" "bool" { default = true }
config "optionalAny" "any" { default = { "key" = "value" } }