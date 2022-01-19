package pulumi

type resourceSet map[Resource]struct{}

func (s resourceSet) add(r Resource) {
	s[r] = struct{}{}
}

func (s resourceSet) any() bool {
	return len(s) > 0
}

func (s resourceSet) delete(r Resource) {
	delete(s, r)
}

func (s resourceSet) has(r Resource) bool {
	_, ok := s[r]
	return ok
}
