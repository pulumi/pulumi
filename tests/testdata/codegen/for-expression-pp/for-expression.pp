names = ["alpha", "beta", "gamma"]

output prefixed {
	value = [for n in names : "prefix-${n}"]
}

output filtered {
	value = [for n in names : n if n != "beta"]
}

output indexed {
	value = [for i, n in names : "${i}:${n}"]
}
