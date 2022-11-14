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