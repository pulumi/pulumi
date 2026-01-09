data = [for entry in entries([1,2,3]) : {
    usingKey: entry.key
    usingValue: entry.value
}]