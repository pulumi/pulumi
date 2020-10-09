package provider

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
)

type Logger interface {
	Debugf(f string, args ...interface{})
	Infof(f string, args ...interface{})
	Errorf(f string, args ...interface{})
	Warningf(f string, args ...interface{})
}

type logger struct {
	logFunc func(severity diag.Severity, urn resource.URN, msg string, streamID int32)

	urn      resource.URN
	streamID int32
}

func (l *logger) logf(severity diag.Severity, f string, args ...interface{}) {
	l.logFunc(severity, l.urn, fmt.Sprintf(f, args...), l.streamID)
}

func (l *logger) Debugf(f string, args ...interface{}) {
	l.logf(diag.Debug, f, args...)
}

func (l *logger) Infof(f string, args ...interface{}) {
	l.logf(diag.Info, f, args...)
}

func (l *logger) Errorf(f string, args ...interface{}) {
	l.logf(diag.Error, f, args...)
}

func (l *logger) Warningf(f string, args ...interface{}) {
	l.logf(diag.Warning, f, args...)
}

type Context struct {
	context.Context

	host plugin.Host
	urn  resource.URN
}

func NewContext(ctx context.Context, host plugin.Host, urn resource.URN) *Context {
	return &Context{Context: ctx, host: host, urn: urn}
}

func (ctx *Context) Log() Logger {
	return ctx.LogStream(0)
}

func (ctx *Context) LogStream(streamID int) Logger {
	return &logger{logFunc: ctx.host.Log, urn: ctx.urn, streamID: int32(streamID)}
}

func (ctx *Context) Status() Logger {
	return ctx.StatusStream(0)
}

func (ctx *Context) StatusStream(streamID int) Logger {
	return &logger{logFunc: ctx.host.LogStatus, urn: ctx.urn, streamID: int32(streamID)}
}
