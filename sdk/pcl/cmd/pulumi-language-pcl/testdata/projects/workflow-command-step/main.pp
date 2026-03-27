step "touch-file" {
  input_type = {
    input_file = string
  }
  command = "touch \"$input_file\""
}
