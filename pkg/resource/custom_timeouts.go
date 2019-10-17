package resource

type CustomTimeouts struct {
	Create float64 `json:"create,omitempty" yaml:"create,omitempty"`
	Update float64 `json:"update,omitempty" yaml:"update,omitempty"`
	Delete float64 `json:"delete,omitempty" yaml:"delete,omitempty"`
}

func (c *CustomTimeouts) IsNotEmpty() bool {
	return c.Delete != 0 || c.Update != 0 || c.Create != 0
}
