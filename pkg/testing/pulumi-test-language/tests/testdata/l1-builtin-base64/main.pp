config "input" "string" { }

bytes = fromBase64(input)

output "data" {
    value = bytes
}

output "roundtrip" { 
    value = toBase64(bytes)
}