resource rt "synthetic:resourceProperties:Root" {
}

output trivial {
    value = rt
}
output simple {
    value = rt.res1
}
output foo {
    value = rt.res1.obj1.res2.obj2
}
output complex {
    value = rt.res1.obj1.res2.obj2.answer
}
