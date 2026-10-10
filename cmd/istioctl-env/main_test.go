package main

import (
	"reflect"
	"testing"
	"time"
)

// recordedSet captures env writes performed by applyGlobalFlags without
// mutating process state. It implements the setenvFn signature.
type recordedSet struct {
	entries map[string]string
}

func newRecorded() *recordedSet {
	return &recordedSet{entries: map[string]string{}}
}

func (r *recordedSet) set(key, value string) error {
	r.entries[key] = value
	return nil
}

func TestApplyGlobalFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantArgs []string
		wantEnv  map[string]string
		wantErr  bool
	}{
		{
			name:     "no flags",
			args:     []string{"install", "1.24.0"},
			wantArgs: []string{"install", "1.24.0"},
			wantEnv:  map[string]string{},
		},
		{
			name:     "offline only",
			args:     []string{"--offline", "list-remote"},
			wantArgs: []string{"list-remote"},
			wantEnv:  map[string]string{"ISTIOENV_OFFLINE": "1"},
		},
		{
			name:     "token with space form",
			args:     []string{"--github-token", "ghp_abc", "install"},
			wantArgs: []string{"install"},
			wantEnv:  map[string]string{"ISTIOENV_GITHUB_TOKEN": "ghp_abc"},
		},
		{
			name:     "token with = form",
			args:     []string{"--github-token=ghp_abc", "install"},
			wantArgs: []string{"install"},
			wantEnv:  map[string]string{"ISTIOENV_GITHUB_TOKEN": "ghp_abc"},
		},
		{
			name:     "api and download mirror together",
			args:     []string{"--api-mirror", "https://api.corp", "--download-mirror=https://dl.corp", "install", "1.24.0"},
			wantArgs: []string{"install", "1.24.0"},
			wantEnv: map[string]string{
				"ISTIOENV_API_MIRROR":      "https://api.corp",
				"ISTIOENV_DOWNLOAD_MIRROR": "https://dl.corp",
			},
		},
		{
			name:     "flags can appear after the subcommand",
			args:     []string{"install", "--offline", "1.24.0", "--github-token=t"},
			wantArgs: []string{"install", "1.24.0"},
			wantEnv: map[string]string{
				"ISTIOENV_OFFLINE":      "1",
				"ISTIOENV_GITHUB_TOKEN": "t",
			},
		},
		{
			name:     "double-dash stops parsing",
			args:     []string{"exec", "1.24.0", "--", "--github-token", "notaflag"},
			wantArgs: []string{"exec", "1.24.0", "--", "--github-token", "notaflag"},
			wantEnv:  map[string]string{},
		},
		{
			name:    "bare --github-token with no value is an error",
			args:    []string{"install", "--github-token"},
			wantErr: true,
		},
		{
			name:    "bare --api-mirror with no value is an error",
			args:    []string{"--api-mirror"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := newRecorded()
			got, err := applyGlobalFlags(tt.args, rec.set)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.wantArgs) {
				t.Errorf("args mismatch: got %#v want %#v", got, tt.wantArgs)
			}
			if !reflect.DeepEqual(rec.entries, tt.wantEnv) {
				t.Errorf("env mismatch: got %#v want %#v", rec.entries, tt.wantEnv)
			}
		})
	}
}

func TestParseDurationShorthand(t *testing.T) {
	const day = 24 * time.Hour
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "30d", want: 30 * day},
		{in: "2w", want: 14 * day},
		{in: "1w3d", want: 10 * day},
		{in: "2W", want: 14 * day},
		{in: " 7d ", want: 7 * day},
		{in: "12h30m", want: 12*time.Hour + 30*time.Minute},
		{in: "0s", want: 0},
		{in: "", wantErr: true},
		{in: "   ", wantErr: true},
		{in: "bogus", wantErr: true},
		{in: "30x", wantErr: true},
		{in: "30", wantErr: true},
		{in: "d", wantErr: true},
		{in: "30d12h", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok, err := parseDurationShorthand(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if ok {
					t.Fatal("ok should be false on error")
				}
				return
			}
			if !ok {
				t.Fatal("ok should be true for parsed input")
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstArg(t *testing.T) {
	if got := firstArg([]string{"global"}); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
	if got := firstArg([]string{"global", "1.24.0", "extra"}); got != "1.24.0" {
		t.Fatalf("got %q, want 1.24.0", got)
	}
}
