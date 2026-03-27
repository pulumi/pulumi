step "touch-file" {
  command = "touch \"$input_file\""
}

workflow "main" {
  job "build" {
    step "touch-file" {
      uses = "touch-file"
    }
  }
}
