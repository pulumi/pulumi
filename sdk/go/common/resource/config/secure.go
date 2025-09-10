package config

// A "reference" to where a secure value is located
type secureLocationRef struct {
	container any // pointer to slice or map
	key       any // string (for map) or int (for slice)
}

func collectSecureFromKeyMap(objectMap map[Key]object, locationRefs *[]secureLocationRef, values *[]string) {
	for k, obj := range objectMap {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectSecureFromKeyMap(value, locationRefs, values)
		case map[string]object:
			collectSecureFromStringMap(value, locationRefs, values)
		case []object:
			collectSecureFromArray(value, locationRefs, values)
		case string:
			if obj.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: objectMap, key: k})
				*values = append(*values, value)
			}
			continue
		}
	}
}

func collectSecureFromStringMap(objectMap map[string]object, locationRefs *[]secureLocationRef, values *[]string) {
	for k, obj := range objectMap {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectSecureFromKeyMap(value, locationRefs, values)
		case map[string]object:
			collectSecureFromStringMap(value, locationRefs, values)
		case []object:
			collectSecureFromArray(value, locationRefs, values)
		case string:
			if obj.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: objectMap, key: k})
				*values = append(*values, value)
			}
			continue
		}
	}
}

func collectSecureFromArray(objectArray []object, locationRefs *[]secureLocationRef, values *[]string) {
	for i, obj := range objectArray {
		switch value := obj.value.(type) {
		case map[Key]object:
			collectSecureFromKeyMap(value, locationRefs, values)
		case map[string]object:
			collectSecureFromStringMap(value, locationRefs, values)
		case []object:
			collectSecureFromArray(value, locationRefs, values)
		case string:
			if obj.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: objectArray, key: i})
				*values = append(*values, value)
			}
			continue
		}
	}
}

func collectSecureFromPlaintextKeyMap(plaintextMap map[Key]plaintext, locationRefs *[]secureLocationRef, values *[]string) {
	for k, pt := range plaintextMap {
		switch value := pt.value.(type) {
		case map[Key]plaintext:
			collectSecureFromPlaintextKeyMap(value, locationRefs, values)
		case map[string]plaintext:
			collectSecureFromPlaintextStringMap(value, locationRefs, values)
		case []plaintext:
			collectSecureFromPlaintextArray(value, locationRefs, values)
		case string:
			if pt.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: plaintextMap, key: k})
				*values = append(*values, value)
			}
			continue
		}
	}
}

func collectSecureFromPlaintextStringMap(plaintextMap map[string]plaintext, locationRefs *[]secureLocationRef, values *[]string) {
	for k, pt := range plaintextMap {
		switch value := pt.value.(type) {
		case map[Key]plaintext:
			collectSecureFromPlaintextKeyMap(value, locationRefs, values)
		case map[string]plaintext:
			collectSecureFromPlaintextStringMap(value, locationRefs, values)
		case []plaintext:
			collectSecureFromPlaintextArray(value, locationRefs, values)
		case string:
			if pt.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: plaintextMap, key: k})
				*values = append(*values, value)
			}
			continue
		}
	}
}

func collectSecureFromPlaintextArray(plaintextArray []plaintext, locationRefs *[]secureLocationRef, values *[]string) {
	for i, pt := range plaintextArray {
		switch value := pt.value.(type) {
		case map[Key]plaintext:
			collectSecureFromPlaintextKeyMap(value, locationRefs, values)
		case map[string]plaintext:
			collectSecureFromPlaintextStringMap(value, locationRefs, values)
		case []plaintext:
			collectSecureFromPlaintextArray(value, locationRefs, values)
		case string:
			if pt.secure {
				*locationRefs = append(*locationRefs, secureLocationRef{container: plaintextArray, key: i})
				*values = append(*values, value)
			}
			continue
		}
	}
}
