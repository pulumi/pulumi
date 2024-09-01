package plugin

import (
	"context"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	pbempty "google.golang.org/protobuf/types/known/emptypb"
)

type AnalyzerGRPCWrapperFunc func(analyzer Analyzer) (pulumirpc.AnalyzerServer, error)

func NewAnalyzerPluginProxy(analyzer Analyzer) (pulumirpc.AnalyzerServer, error) {
	return &analyzerServer{
		analyzer: analyzer,
	}, nil
}

type analyzerServer struct {
	pulumirpc.UnimplementedAnalyzerServer
	analyzer Analyzer
}

func (a *analyzerServer) Analyze(ctx context.Context, req *pulumirpc.AnalyzeRequest) (*pulumirpc.AnalyzeResponse, error) {
	rsrc, err := FromProtoAnalyzerRequest(req)
	if err != nil {
		return nil, err
	}
	diagnostics, err := a.analyzer.Analyze(*rsrc)
	if err != nil {
		return nil, err
	}
	res, err := ToProtoAnalyzerResponse(diagnostics)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (a *analyzerServer) AnalyzeStack(ctx context.Context, req *pulumirpc.AnalyzeStackRequest) (
	*pulumirpc.AnalyzeResponse, error) {
	rs, err := FromProtoAnalyzerStackRequest(req)
	if err != nil {
		return nil, err
	}
	diagnostics, err := a.analyzer.AnalyzeStack(rs)
	if err != nil {
		return nil, err
	}
	res, err := ToProtoAnalyzerResponse(diagnostics)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (a *analyzerServer) Remediate(ctx context.Context, req *pulumirpc.AnalyzeRequest) (*pulumirpc.RemediateResponse, error) {
	rsrc, err := FromProtoAnalyzerRequest(req)
	if err != nil {
		return nil, err
	}
	remediations, err := a.analyzer.Remediate(*rsrc)
	if err != nil {
		return nil, err
	}
	rems, err := ToProtoRemediateResponse(remediations)
	if err != nil {
		return nil, err
	}
	return rems, nil
}

func (a *analyzerServer) GetAnalyzerInfo(context.Context, *pbempty.Empty) (*pulumirpc.AnalyzerInfo, error) {
	info, err := a.analyzer.GetAnalyzerInfo()
	if err != nil {
		return nil, err
	}
	ai, err := ToProtoAnalyzerInfo(&info)
	if err != nil {
		return nil, err
	}
	return ai, nil
}

func (a *analyzerServer) GetPluginInfo(ctx context.Context, _ *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	info, err := a.analyzer.GetPluginInfo()
	if err != nil {
		return nil, err
	}
	return ToProtoPluginInfo(&info), nil
}

func (a *analyzerServer) Configure(ctx context.Context, req *pulumirpc.ConfigureAnalyzerRequest) (*pbempty.Empty,
	error) {
	conf, err := FromProtoConfigureAnalyzerRequest(req)
	if err != nil {
		return nil, err
	}
	err = a.analyzer.Configure(conf)
	if err != nil {
		return nil, err
	}
	return &pbempty.Empty{}, err
}
