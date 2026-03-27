step "echo" {
  input_type  = "bool"
  output_type = "bool"
  expr        = "!input"
}

workflow "main" {
  job "build" {
    step "echo" {
      uses = "echo"
    }
  }
}
