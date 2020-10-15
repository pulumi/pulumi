package provider

type ErrNotFound interface {
	isNotFound()
}

type errNotFound string

func (errNotFound) isNotFound() {}

func IsNotFound(err error) bool {
	_, ok := err.(ErrNotFound)
	return ok
}

func NotFound(message string) ErrNotFound {
	return errNotFound(message)
}
