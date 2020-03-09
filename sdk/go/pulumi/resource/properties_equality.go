package resource

// DeepEquals returns true if this property map is deeply equal to the other property map; and false otherwise.
func (props PropertyMap) DeepEquals(other PropertyMap) bool {
	// If any in props either doesn't exist, or is of a different value, return false.
	for _, k := range props.StableKeys() {
		v := props[k]
		if p, has := other[k]; has {
			if !v.DeepEquals(p) {
				return false
			}
		} else if v.HasValue() {
			return false
		}
	}

	// If the other map has properties that this map doesn't have, return false.
	for _, k := range other.StableKeys() {
		if _, has := props[k]; !has && other[k].HasValue() {
			return false
		}
	}

	return true
}

// DeepEquals returns true if this property map is deeply equal to the other property map; and false otherwise.
func (v PropertyValue) DeepEquals(other PropertyValue) bool {
	// Arrays are equal if they are both of the same size and elements are deeply equal.
	if v.IsArray() {
		if !other.IsArray() {
			return false
		}
		va := v.ArrayValue()
		oa := other.ArrayValue()
		if len(va) != len(oa) {
			return false
		}
		for i, elem := range va {
			if !elem.DeepEquals(oa[i]) {
				return false
			}
		}
		return true
	}

	// Assets and archives enjoy value equality.
	if v.IsAsset() {
		if !other.IsAsset() {
			return false
		}
		return v.AssetValue().Equals(other.AssetValue())
	} else if v.IsArchive() {
		if !other.IsArchive() {
			return false
		}
		return v.ArchiveValue().Equals(other.ArchiveValue())
	}

	// Object values are equal if their contents are deeply equal.
	if v.IsObject() {
		if !other.IsObject() {
			return false
		}
		vo := v.ObjectValue()
		oa := other.ObjectValue()
		return vo.DeepEquals(oa)
	}

	// Secret are equal if the value they wrap are equal.
	if v.IsSecret() {
		if !other.IsSecret() {
			return false
		}
		vs := v.SecretValue()
		os := other.SecretValue()

		return vs.Element.DeepEquals(os.Element)
	}

	// For all other cases, primitives are equal if their values are equal.
	return v.V == other.V
}
