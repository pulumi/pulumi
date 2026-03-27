step "touch-file" {
  input_type = "workflow:index:CommandStepInput"
  command = "touch \"$input_file\""
}
