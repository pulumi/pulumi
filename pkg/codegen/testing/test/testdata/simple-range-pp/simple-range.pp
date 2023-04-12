resource numbers "random:index/randomInteger:RandomInteger" {
	options {
		range = 2
	}

	min = 1
	max = range.value
	seed = "seed${range.value}"
}

output first { value = numbers[0].id }
output second { value = numbers[1].id }
