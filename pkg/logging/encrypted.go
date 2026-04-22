// Copyright 2026, Pulumi Corporation.
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

// Package logging provides automatic logging to disk, encrypted when a secrets manager
// is available, plain gzip otherwise.
package logging

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/engine/encryptedlog"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

var currentLogger *Logger

// UpgradeCurrentLogger upgrades the currently active logger to encrypted mode
// and renames the log file to include the stack name and update ID.
func UpgradeCurrentLogger(ctx context.Context, stackName, updateID string, sm secrets.Manager) error {
	if currentLogger == nil {
		return nil
	}
	return currentLogger.UpgradeToEncrypted(ctx, stackName, updateID, sm)
}

// RenameCurrentLogger renames the current log file to include the given
// stack name and update ID. This is useful when the update ID becomes
// available after the logger has already been upgraded to encrypted mode.
func RenameCurrentLogger(stackName, updateID string) error {
	if currentLogger == nil {
		return nil
	}
	return currentLogger.rename(stackName, updateID)
}

// Logger captures output to a log file on disk, optionally encrypted.
type Logger struct {
	mu        sync.Mutex
	sink      io.WriteCloser // current sink (gzipSink or EncryptedLogWriter)
	handler   *slog.JSONHandler
	f         *os.File
	filePath  string
	encrypted bool
}

// StartLogging creates a log file under ~/.pulumi/logs/ and installs it as
// the logging sink. If sm is non-nil the log is encrypted; otherwise it
// falls back to gzip until UpgradeToEncrypted is called.
func StartLogging(
	ctx context.Context,
	sm secrets.Manager,
) (*Logger, error) {
	logsDir, err := workspace.GetPulumiPath("logs")
	if err != nil {
		return nil, fmt.Errorf("getting log directory: %w", err)
	}
	if err := os.MkdirAll(logsDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	RotateLogs(logsDir)

	ts := time.Now().Format("20060102T150405")
	name := fmt.Sprintf("pulumi-%s-%d.log", ts, os.Getpid())
	filePath := filepath.Join(logsDir, name)

	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}

	l := &Logger{
		f:        f,
		filePath: filePath,
	}

	sink, encErr := newEncryptedSink(ctx, f, sm)
	if encErr != nil {
		l.sink = newGzipSink(f)
	} else {
		l.sink = sink
		l.encrypted = true
	}

	l.handler = slog.NewJSONHandler(l, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logging.SetSinkHandler(l.handler)
	currentLogger = l
	return l, nil
}

// Write implements io.Writer, forwarding to the current sink under the mutex.
// The slog.JSONHandler writes through this method, which ensures it always
// targets the current sink even after an upgrade from gzip to encrypted.
func (l *Logger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sink.Write(p)
}

// UpgradeToEncrypted switches from gzip to encrypted logging, re-encrypting
// any data already written. No-op if already encrypted or sm is nil.
func (l *Logger) UpgradeToEncrypted(ctx context.Context, stackName, updateID string, sm secrets.Manager) error {
	if l == nil || sm == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.encrypted {
		return nil
	}

	if stackName != "" {
		if err := l.renameLocked(stackName, updateID); err != nil {
			return fmt.Errorf("renaming log file: %w", err)
		}
	}

	if err := l.sink.Close(); err != nil {
		return fmt.Errorf("flushing gzip sink: %w", err)
	}
	if err := l.f.Sync(); err != nil {
		return fmt.Errorf("syncing log file: %w", err)
	}

	if _, err := l.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}
	gz, err := gzip.NewReader(l.f)
	if err != nil {
		return fmt.Errorf("reading gzip data: %w", err)
	}
	plaintext, err := io.ReadAll(gz)
	if err != nil {
		return fmt.Errorf("decompressing log data: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("closing gzip reader: %w", err)
	}

	if err := l.f.Truncate(0); err != nil {
		return fmt.Errorf("truncating log file: %w", err)
	}
	if _, err := l.f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking after truncate: %w", err)
	}

	encSink, err := encryptedlog.NewWriter(ctx, l.f, sm.Encrypter())
	if err != nil {
		l.sink = newGzipSink(l.f)
		return fmt.Errorf("creating encrypted writer: %w", err)
	}

	if len(plaintext) > 0 {
		if _, err := encSink.Write(plaintext); err != nil {
			return fmt.Errorf("writing old logs to encrypted sink: %w", err)
		}
	}

	l.sink = encSink
	l.encrypted = true
	return nil
}

// Close flushes buffered data and closes the underlying file.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	logging.SetSinkHandler(nil)
	if currentLogger == l {
		currentLogger = nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	sinkErr := l.sink.Close()
	fileErr := l.f.Close()
	if sinkErr != nil {
		return sinkErr
	}
	return fileErr
}

// FilePath returns the path to the log file.
func (l *Logger) FilePath() string {
	if l == nil {
		return ""
	}
	return l.filePath
}

func (l *Logger) rename(stackName, updateID string) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.renameLocked(stackName, updateID)
}

// renameLocked renames the log file. Must be called with l.mu held.
func (l *Logger) renameLocked(stackName, updateID string) error {
	dir := filepath.Dir(l.filePath)
	ts := time.Now().Format("20060102T150405")
	safeName := strings.ReplaceAll(stackName, "/", "+")
	name := safeName + "-" + ts
	if updateID != "" {
		name += "-" + updateID
	}
	name += ".log"
	newPath := filepath.Join(dir, name)

	if err := os.Rename(l.filePath, newPath); err != nil { //nolint:forbidigo // same directory rename
		return err
	}
	l.filePath = newPath
	return nil
}

func newEncryptedSink(ctx context.Context, w io.Writer, sm secrets.Manager) (io.WriteCloser, error) {
	if sm == nil {
		return nil, errors.New("no secrets manager available")
	}
	return encryptedlog.NewWriter(ctx, w, sm.Encrypter())
}

type gzipSink struct {
	gz *gzip.Writer
}

func newGzipSink(w io.Writer) io.WriteCloser {
	return &gzipSink{gz: gzip.NewWriter(w)}
}

func (g *gzipSink) Write(p []byte) (int, error) {
	return g.gz.Write(p)
}

func (g *gzipSink) Close() error {
	return g.gz.Close()
}
