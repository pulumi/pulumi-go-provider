// Copyright 2022-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"log/slog"

	pprovider "github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"

	"github.com/pulumi/pulumi-go-provider/internal/key"
)

type Logger struct {
	ctx   context.Context
	inner logSink
	urn   resource.URN
}

func (l Logger) Debug(msg string)                  { l.inner.Log(l.ctx, l.urn, diag.Debug, msg) }
func (l Logger) Debugf(msg string, a ...any)       { l.Debug(fmt.Sprintf(msg, a...)) }
func (l Logger) DebugStatus(msg string)            { l.inner.LogStatus(l.ctx, l.urn, diag.Debug, msg) }
func (l Logger) DebugStatusf(msg string, a ...any) { l.DebugStatus(fmt.Sprintf(msg, a...)) }

func (l Logger) Info(msg string)                  { l.inner.Log(l.ctx, l.urn, diag.Info, msg) }
func (l Logger) Infof(msg string, a ...any)       { l.Info(fmt.Sprintf(msg, a...)) }
func (l Logger) InfoStatus(msg string)            { l.inner.LogStatus(l.ctx, l.urn, diag.Info, msg) }
func (l Logger) InfoStatusf(msg string, a ...any) { l.InfoStatus(fmt.Sprintf(msg, a...)) }

func (l Logger) Warning(msg string)                  { l.inner.Log(l.ctx, l.urn, diag.Warning, msg) }
func (l Logger) Warningf(msg string, a ...any)       { l.Warning(fmt.Sprintf(msg, a...)) }
func (l Logger) WarningStatus(msg string)            { l.inner.LogStatus(l.ctx, l.urn, diag.Warning, msg) }
func (l Logger) WarningStatusf(msg string, a ...any) { l.WarningStatus(fmt.Sprintf(msg, a...)) }

func (l Logger) Error(msg string)                  { l.inner.Log(l.ctx, l.urn, diag.Error, msg) }
func (l Logger) Errorf(msg string, a ...any)       { l.Error(fmt.Sprintf(msg, a...)) }
func (l Logger) ErrorStatus(msg string)            { l.inner.LogStatus(l.ctx, l.urn, diag.Error, msg) }
func (l Logger) ErrorStatusf(msg string, a ...any) { l.ErrorStatus(fmt.Sprintf(msg, a...)) }

type logSink interface {
	// Log logs a global message, including errors and warnings.
	Log(context.Context, resource.URN, diag.Severity, string)
	// LogStatus logs a global status message, including errors and warnings. Status messages will
	// appear in the `Info` column of the progress display, but not in the final output.
	LogStatus(context.Context, resource.URN, diag.Severity, string)
}

func GetLogger(ctx context.Context) Logger {
	var (
		sink logSink = slogSink{}
		urn  resource.URN
	)
	if v := ctx.Value(key.Logger); v != nil {
		sink = v.(logSink)
	}
	if v := ctx.Value(key.URN); v != nil {
		urn = v.(resource.URN)
	}
	return Logger{ctx, sink, urn}
}

var (
	_ logSink = (*hostSink)(nil)
	_ logSink = (*slogSink)(nil)
)

type hostSink struct{ host *pprovider.HostClient }

func (h hostSink) Log(ctx context.Context, urn resource.URN, severity diag.Severity, msg string) {
	err := h.host.Log(ctx, severity, urn, msg)
	if err != nil {
		slog := slog.Default().With("hostLogFailed", err.Error())
		slogSink{}.log(ctx, slog, urn, severity, msg)
	}
}

func (h hostSink) LogStatus(ctx context.Context, urn resource.URN, severity diag.Severity, msg string) {
	err := h.host.LogStatus(ctx, severity, urn, msg)
	if err != nil {
		slog := slog.Default().With(
			"hostLogFailed", err.Error(),
			"kind", "status",
		)
		slogSink{}.log(ctx, slog, urn, severity, msg)
	}
}

type slogSink struct{}

func (slogSink) log(ctx context.Context, slog *slog.Logger, urn resource.URN, severity diag.Severity, msg string) {
	log := slog.InfoContext // We default to Info as the log level
	switch severity {
	case diag.Debug:
		log = slog.DebugContext
	case diag.Warning:
		log = slog.WarnContext
	case diag.Error:
		log = slog.ErrorContext
	}
	log(ctx, msg, "urn", string(urn))
}

func (s slogSink) Log(ctx context.Context, urn resource.URN, severity diag.Severity, msg string) {
	s.log(ctx, slog.Default(), urn, severity, msg)
}

func (s slogSink) LogStatus(ctx context.Context, urn resource.URN, severity diag.Severity, msg string) {
	s.log(ctx, slog.Default().With("kind", "status"), urn, severity, msg)
}
