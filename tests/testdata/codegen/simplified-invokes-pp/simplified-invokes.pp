everyArg = invoke("std:index:AbsMultiArgs", {
    a: 10
    b: 20
    c: 30
})

onlyRequiredArgs = invoke("std:index:AbsMultiArgs", {
    a: 10
})

optionalArgs = invoke("std:index:AbsMultiArgs", {
   a: 10
   c: 30
})

nestedUse = invoke("std:index:AbsMultiArgs", {
    a: everyArg
    b: invoke("std:index:AbsMultiArgs", { a: 42 })
})

output result {
    value = nestedUse
}