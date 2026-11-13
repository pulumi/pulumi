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

package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/gofrs/uuid"
	"github.com/rogpeppe/go-internal/lockedfile"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
)

const (
	configFile    = "telemetry.json"
	configVersion = 1
)

var notice = colors.SpecImportant + "notice:" + colors.Reset +
	" The Pulumi CLI collects anonymous usage telemetry to help improve Pulumi. " +
	"No telemetry has been sent yet. To opt out, run `pulumi telemetry disable`, " +
	"or set PULUMI_TELEMETRY=0 or DO_NOT_TRACK=1."

type Mode string

const (
	ModeEnabled  Mode = "enabled"
	ModeDisabled Mode = "disabled"
)

type Source string

const (
	SourceEnv        Source = "env"
	SourceConfig     Source = "config"
	SourceDoNotTrack Source = "DO_NOT_TRACK"
	SourceDefault    Source = "default"
)

type Settings struct {
	Mode       Mode
	ModeSource Source
	Log        bool
	LogSource  Source
	LogPath    string
}

type config struct {
	Version      int    `json:"version"`
	Installation string `json:"installation"`
	Telemetry    string `json:"telemetry,omitempty"`
	Log          bool   `json:"log,omitempty"`
	Notified     bool   `json:"notified,omitempty"`
}

func newTelemetry(args InitArgs) *telemetry {
	telemetryEnv := args.Env.GetString(env.Telemetry)
	logging := cmdutil.IsTruthy(args.Env.GetString(env.TelemetryLog))
	if telemetryEnv != "" && !cmdutil.IsTruthy(telemetryEnv) && !logging {
		return nil
	}
	optedOut := telemetryEnv == "" && args.Env.GetBool(env.DoNotTrack) && !logging
	warn := func(err error) {
		if !optedOut {
			args.Diags.Warningf(diag.Message("", "Telemetry is disabled: %v"), err)
		}
	}

	path := filepath.Join(args.HomeDir, configFile)
	cfg, exists, err := readConfig(path)
	if err != nil {
		warn(err)
		return nil
	}
	s, err := cfg.settings(path, args.HomeDir, args.Env)
	if err != nil {
		warn(err)
		return nil
	}
	notify := s.notify(cfg)
	if s.Mode == ModeDisabled && !s.Log {
		return nil
	}

	if !exists || notify {
		err = os.MkdirAll(args.HomeDir, 0o700)
		if err == nil {
			err = lockedfile.Transform(path, func(old []byte) ([]byte, error) {
				var err error
				if cfg, err = parseConfig(path, old); err != nil {
					return nil, err
				}
				if s, err = cfg.settings(path, args.HomeDir, args.Env); err != nil {
					return nil, err
				}
				notify = s.notify(cfg)
				if len(old) != 0 && !notify {
					return old, nil
				}
				cfg.Notified = cfg.Notified || notify
				return marshalConfig(cfg)
			})
		}
		if err != nil {
			warn(err)
			return nil
		}
	}

	if notify {
		args.Diags.Infoerrf(diag.RawMessage("", notice))
	}
	send := s.Mode == ModeEnabled && !notify
	if !send && !s.Log {
		return nil
	}

	invocation, err := uuid.NewV4()
	if err != nil {
		warn(err)
		return nil
	}

	t := &telemetry{
		installation: cfg.Installation,
		invocation:   invocation.String(),
		send:         send,
	}
	if s.Log {
		t.logPath = s.LogPath
		if err := os.MkdirAll(filepath.Dir(t.logPath), 0o700); err != nil {
			args.Diags.Warningf(diag.Message("", "Could not create the telemetry log: %v"), err)
			t.logPath = ""
		}
	}
	if send {
		pulse := args.Pulse
		if pulse <= 0 {
			pulse = defaultPulse
		}
		t.queue = newQueue(filepath.Join(args.HomeDir, "telemetry"), t.invocation, args.URL, args.UserAgent,
			args.CI, pulse)
		t.queue.held = true
		if !args.CI {
			t.queue.wal = openWAL(t.queue.dir, t.invocation)
		}
	}
	if t.logPath == "" && !send {
		return nil
	}
	return t
}

func readConfig(path string) (config, bool, error) {
	data, err := lockedfile.Read(path)
	if errors.Is(err, fs.ErrNotExist) || err == nil && len(data) == 0 {
		return config{}, false, nil
	} else if err != nil {
		return config{}, false, err
	}
	cfg, err := parseConfig(path, data)
	return cfg, true, err
}

func parseConfig(path string, data []byte) (config, error) {
	if len(data) == 0 {
		id, err := uuid.NewV4()
		return config{Version: configVersion, Installation: id.String()}, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("could not parse %s: %w", path, err)
	}
	if cfg.Version != configVersion {
		return cfg, fmt.Errorf("%s has unsupported version %d", path, cfg.Version)
	}
	if _, err := uuid.FromString(cfg.Installation); err != nil {
		return cfg, fmt.Errorf("%s has an invalid installation ID", path)
	}
	return cfg, nil
}

func marshalConfig(cfg config) ([]byte, error) {
	data, err := json.MarshalIndent(cfg, "", "    ")
	return append(data, '\n'), err
}

func (cfg config) settings(path, homeDir string, e declared.Env) (Settings, error) {
	s := Settings{LogPath: filepath.Join(homeDir, "telemetry", "log", "events.jsonl")}

	switch raw := e.GetString(env.Telemetry); {
	case raw != "":
		s.Mode, s.ModeSource = ModeDisabled, SourceEnv
		if cmdutil.IsTruthy(raw) {
			s.Mode = ModeEnabled
		}
	case cfg.Telemetry != "":
		s.Mode, s.ModeSource = Mode(cfg.Telemetry), SourceConfig
		if s.Mode != ModeEnabled && s.Mode != ModeDisabled {
			return s, fmt.Errorf("%s has unrecognized telemetry setting %q", path, cfg.Telemetry)
		}
	case e.GetBool(env.DoNotTrack):
		s.Mode, s.ModeSource = ModeDisabled, SourceDoNotTrack
	default:
		s.Mode, s.ModeSource = ModeEnabled, SourceDefault
	}

	if raw := e.GetString(env.TelemetryLog); raw != "" {
		s.Log, s.LogSource = cmdutil.IsTruthy(raw), SourceEnv
	} else if cfg.Log {
		s.Log, s.LogSource = true, SourceConfig
	} else {
		s.LogSource = SourceDefault
	}
	return s, nil
}

func (s Settings) notify(cfg config) bool {
	return s.Mode == ModeEnabled && s.ModeSource == SourceDefault && !cfg.Notified
}
