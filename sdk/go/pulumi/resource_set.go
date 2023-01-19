package pulumi

type resourceSet map[Resource]struct{}

func (s resourceSet) add(r Resource) {
	s[r] = struct{}{}
}
