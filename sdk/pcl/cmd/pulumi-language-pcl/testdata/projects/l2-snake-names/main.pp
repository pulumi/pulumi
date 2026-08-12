// Resource inputs are correctly translated
resource "first" "snake_names:cool_module:some_resource" {
    the_input = true
    nested = {
        nested_value = "nested"
    }
}

// Datasource outputs are correctly translated
resource "third" "snake_names:cool_module:another_resource" {
    the_input = invoke("snake_names:cool_module:some_data", {
        the_input = first.the_output["someKey"][0].nested_output
        nested = [{
            value = "fuzz"
        }]
    }).nested_output[0]["key"].value
}
