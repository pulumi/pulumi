# We use `secret` to lift plain values to output space, then check we can index into them

config "anObject" "object({property=string})"  {}
config "anyObject" {}

l = secret([1])
m = secret({"key": true})
c = secret(anObject)
o = secret({property: "value"})
a = secret(anyObject)

output "l" {
  value = l[0]
}

output "m" {
  value = m["key"]
}

output "c" {
  value = c.property
}

output "o" {
  value = o.property
}

output "a" {
  value = a.property
}

