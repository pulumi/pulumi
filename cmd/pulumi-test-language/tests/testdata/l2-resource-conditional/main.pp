resource "resA" "simple:index:Resource" {
    value = true
}

condition "cond" {
    condition = resA.value

    true {
        resource "resB" "simple:index:Resource" {
            value = false
        }
    }
    trueValue = resB.value

    false {
        resource "resC" "simple:index:Resource" {
            value = false
        }
    }
    falseValue = resC.value
}

output "result" {
    value = cond
}
