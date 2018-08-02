package enginetest

import (
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

type ProgramFunc func(runInfo plugin.RunInfo, monitor *ResourceMonitor) error

func NewLanguageRuntime(program ProgramFunc, requiredPlugins ...workspace.PluginInfo) plugin.LanguageRuntime {
	return &languageRuntime{
		requiredPlugins: requiredPlugins,
		program:         program,
	}
}

type languageRuntime struct {
	requiredPlugins []workspace.PluginInfo
	program         ProgramFunc
}

func (p *languageRuntime) Close() error {
	return nil
}

func (p *languageRuntime) GetRequiredPlugins(info plugin.ProgInfo) ([]workspace.PluginInfo, error) {
	return p.requiredPlugins, nil
}

func (p *languageRuntime) Run(info plugin.RunInfo) (string, error) {
	// Connect to the resource monitor and create an appropriate client.
	conn, err := grpc.Dial(info.MonitorAddress, grpc.WithInsecure())
	if err != nil {
		return "", errors.Wrapf(err, "could not connect to resource monitor")
	}

	// Fire up a resource monitor client
	resmon := pulumirpc.NewResourceMonitorClient(conn)

	// Run the program.
	done := make(chan error)
	go func() {
		done <- p.program(info, &ResourceMonitor{resmon: resmon})
	}()
	if progerr := <-done; progerr != nil {
		return progerr.Error(), nil
	}
	return "", nil
}

func (p *languageRuntime) GetPluginInfo() (workspace.PluginInfo, error) {
	return workspace.PluginInfo{Name: "TestLanguage"}, nil
}
