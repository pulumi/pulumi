output "sumTimes" {
    value = (1 + 2) * 3
}

output "minusMinus" {
    value = 10 - (4 - 3)
}

output "notAnd" {
    value = !(true && false)
}

output "negateSum" {
    value = -(1 + 2)
}

output "conditionalIndex" {
    value = (true ? [1] : [2])[0]
}

output "conditionalCondition" {
    value = (true ? false : true) ? "yes" : "no"
}
