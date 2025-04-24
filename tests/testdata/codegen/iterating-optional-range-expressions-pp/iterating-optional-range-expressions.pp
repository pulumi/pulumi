resource "root" "range:index:Root" {}

// creating resources by iterating a property of type array(string) of another resource
resource "fromListOfStrings" "range:index:Example" {
  options {
    range = root.arrayOfString
  }

  someString = range.value
}

// creating resources by iterating a property of type map(string) of another resource
resource "fromMapOfStrings" "range:index:Example" {
  options {
    range = root.mapOfString
  }

  someString = "${range.key} ${range.value}"
}

// computed range list expression to create instances of range:index:Example resource
resource "fromComputedListOfStrings" "range:index:Example" {
  options {
    range = [
        root.mapOfString["hello"],
        root.mapOfString["world"]
    ]
  }

  someString = "${range.key} ${range.value}"
}

// computed range for expression to create instances of range:index:Example resource
resource "fromComputedForExpression" "range:index:Example" {
  options {
    range = [for value in root.arrayOfString : root.mapOfString[value]]
  }

  someString = "${range.key} ${range.value}"
}
