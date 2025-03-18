# We use `secret` to lift plain values to output space, then check we can index into them

config "object" "object({property=string})"  {}

l = secret([1])
m = secret({"key": true})
c = secret(object)
o = secret({property: "value"})

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

