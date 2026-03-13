localValue = "hello"

output "dynamic" {
  value = invoke(
    "any-type-function:index:dynListToDyn",
    {
      inputs = ["hello", localValue, {}]
    }
  ).result
}
