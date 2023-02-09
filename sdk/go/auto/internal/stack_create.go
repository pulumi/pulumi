package internal

type StackCreateConfig struct {
	teams []string
}

func NewStackCreateConfig() *StackCreateConfig {
	return &StackCreateConfig{}
}

func (conf *StackCreateConfig) AddTeam(team string) {
	conf.teams = append(conf.teams)
}

func (conf *StackCreateConfig) GenerateFlags() []string {
	var flags = []string{}
	for _, team := range conf.teams {
		flags = append(flags, "--team", team)
	}
	return flags
}
