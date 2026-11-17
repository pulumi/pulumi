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

// Source is where a telemetry setting came from.
type Source string

const (
	// SourceEnv means the setting was set by an environment variable: PULUMI_TELEMETRY for Enabled,
	// PULUMI_TELEMETRY_LOG for Log.
	SourceEnv Source = "env"
	// SourceConfig means the setting was set in $PULUMI_HOME/telemetry.json, which `pulumi telemetry` writes.
	SourceConfig Source = "config"
	// SourceDoNotTrack means telemetry is disabled because DO_NOT_TRACK is set.
	SourceDoNotTrack Source = "DO_NOT_TRACK"
	// SourceDefault means nothing set the setting, so its default applies.
	SourceDefault Source = "default"
)

// Settings are the effective telemetry settings and where each of them came from.
type Settings struct {
	// Enabled is whether telemetry is sent to the service.
	Enabled bool
	// Where we got the Enabled setting from: PULUMI_TELEMETRY, DO_NOT_TRACK, telemetry.json, or default.
	EnabledSource Source
	// LogPath is the file events are written to, regardless of if they are sent to the service or not. It is empty
	// if events aren't logged.
	LogPath string
	// Where we got the LogPath setting from: PULUMI_TELEMETRY_LOG, telemetry.json, or default.
	LogSource Source
}

// config is the contents of $PULUMI_HOME/telemetry.json.
type config struct {
	// The version of the config file format.
	Version int `json:"version"`
	// The installation ID, a random UUID that identifies this Pulumi installation. It is generated when this file is
	// first written, and stays the same until the file is deleted.
	Installation string `json:"installation"`
	// Whether telemetry is enabled. If nil, the default applies.
	Telemetry *bool `json:"telemetry,omitempty"`
	// Whether telemetry events are logged to $PULUMI_HOME/telemetry/log/events.jsonl.
	Log bool `json:"log,omitempty"`
	// Whether the user has been notified that telemetry is enabled by default and can be disabled.
	Notified bool `json:"notified,omitempty"`
}

func newTelemetry(args InitArgs) *telemetry {
	enabled, enabledSet := lookupBool(args.Env, env.Telemetry)
	logging := args.Env.GetBool(env.TelemetryLog)
	if enabledSet && !enabled && !logging {
		return nil
	}
	// DO_NOT_TRACK can still be overridden by `pulumi telemetry enable`, so the config has to be read, but users
	// who opted out shouldn't be warned when that fails.
	optedOut := !enabledSet && args.Env.GetBool(env.DoNotTrack) && !logging
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
	s := cfg.settings(args.HomeDir, args.Env)
	notify := s.notify(cfg)
	if !s.Enabled && s.LogPath == "" {
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
				s = cfg.settings(args.HomeDir, args.Env)
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
	send := s.Enabled && !notify
	if !send && s.LogPath == "" {
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
	if s.LogPath != "" {
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
		t.queue = newQueue(filepath.Join(args.HomeDir, "telemetry"), t.invocation, args.CI, pulse,
			httpSender(args.URL, args.UserAgent))
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

func (cfg config) settings(homeDir string, e declared.Env) Settings {
	var s Settings

	if enabled, set := lookupBool(e, env.Telemetry); set {
		s.Enabled, s.EnabledSource = enabled, SourceEnv
	} else if cfg.Telemetry != nil {
		s.Enabled, s.EnabledSource = *cfg.Telemetry, SourceConfig
	} else if e.GetBool(env.DoNotTrack) {
		s.EnabledSource = SourceDoNotTrack
	} else {
		s.Enabled, s.EnabledSource = true, SourceDefault
	}

	log := false
	if l, set := lookupBool(e, env.TelemetryLog); set {
		log, s.LogSource = l, SourceEnv
	} else if cfg.Log {
		log, s.LogSource = true, SourceConfig
	} else {
		s.LogSource = SourceDefault
	}
	if log {
		s.LogPath = filepath.Join(homeDir, "telemetry", "log", "events.jsonl")
	}
	return s
}

func (s Settings) notify(cfg config) bool {
	return s.Enabled && s.EnabledSource == SourceDefault && !cfg.Notified
}

func lookupBool(e declared.Env, v declared.BoolValue) (value, set bool) {
	raw, _ := e.GetStore().Raw(v.Var().Name())
	return e.GetBool(v), raw != ""
}
