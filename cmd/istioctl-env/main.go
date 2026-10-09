// Package main is the entry point for the istioctl-env CLI tool.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/user/istioctl-env/internal/commands"
)

// Version is set at build time via ldflags:
//
//	go build -ldflags "-X main.Version=0.1.0"
var Version = "dev"

func main() {
	// Inject version into commands package
	commands.Version = Version

	args, err := applyGlobalFlags(os.Args[1:], os.Setenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if len(args) == 0 {
		commands.Help()
		os.Exit(0)
	}

	switch args[0] {
	case "help", "--help", "-h":
		commands.Help()

	case "version", "--version", "-v":
		commands.PrintVersion()

	case "init":
		shell := ""
		for i := 1; i < len(args); i++ {
			arg := args[i]
			switch {
			case arg == "-h" || arg == "--help":
				commands.InitHelp()
				os.Exit(0)
			case arg == "--shell":
				if i+1 >= len(args) {
					fmt.Fprintln(os.Stderr, "--shell requires a value (bash|zsh|fish)")
					os.Exit(1)
				}
				shell = args[i+1]
				i++
			case strings.HasPrefix(arg, "--shell="):
				shell = strings.TrimPrefix(arg, "--shell=")
			default:
				fmt.Fprintf(os.Stderr, "Unknown argument for init: %s\n", arg)
				commands.InitHelp()
				os.Exit(1)
			}
		}
		err = commands.Init(shell)

	case "list":
		err = commands.List()

	case "list-remote":
		includePrerelease := false
		cached := false
		for _, arg := range args[1:] {
			switch arg {
			case "-h", "--help":
				commands.ListRemoteHelp()
				os.Exit(0)
			case "--prerelease":
				includePrerelease = true
			case "--cached":
				cached = true
			}
		}
		if cached {
			err = commands.ListRemoteCached(includePrerelease)
		} else {
			err = commands.ListRemote(includePrerelease)
		}

	case "latest":
		includePrerelease := false
		for _, arg := range args[1:] {
			switch arg {
			case "-h", "--help":
				commands.LatestHelp()
				os.Exit(0)
			case "--prerelease":
				includePrerelease = true
			}
		}
		err = commands.Latest(includePrerelease)

	case "install":
		version := ""
		silent := false
		for _, arg := range args[1:] {
			if arg == "-s" || arg == "--silent" {
				silent = true
			} else if arg == "-h" || arg == "--help" {
				commands.InstallHelp()
				os.Exit(0)
			} else if version == "" && !strings.HasPrefix(arg, "-") {
				version = arg
			}
		}
		err = commands.Install(version, silent)

	case "uninstall":
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		err = commands.Uninstall(version)

	case "shell":
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		err = commands.Shell(version)

	case "local":
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		err = commands.Local(version)

	case "global":
		version := ""
		if len(args) > 1 {
			version = args[1]
		}
		err = commands.Global(version)

	case "which":
		err = commands.Which()

	case "upgrade":
		err = commands.Upgrade()

	case "autocompletion":
		shell := ""
		for i := 1; i < len(args); i++ {
			arg := args[i]
			switch {
			case arg == "-h" || arg == "--help":
				commands.AutocompletionHelp()
				os.Exit(0)
			case arg == "--shell":
				if i+1 >= len(args) {
					fmt.Fprintln(os.Stderr, "--shell requires a value (bash|zsh|fish)")
					os.Exit(1)
				}
				shell = args[i+1]
				i++
			case strings.HasPrefix(arg, "--shell="):
				shell = strings.TrimPrefix(arg, "--shell=")
			default:
				fmt.Fprintf(os.Stderr, "Unknown argument for autocompletion: %s\n", arg)
				commands.AutocompletionHelp()
				os.Exit(1)
			}
		}
		err = commands.Autocompletion(shell)

	case "status":
		err = commands.Status()

	case "exec":
		version := ""
		execArgs := []string{}
		if len(args) > 1 {
			version = args[1]
			if len(args) > 2 {
				execArgs = args[2:]
			}
		}
		err = commands.Exec(version, execArgs)

	case "doctor":
		fix := false
		deep := false
		for _, arg := range args[1:] {
			switch arg {
			case "-h", "--help":
				commands.DoctorHelp()
				os.Exit(0)
			case "--fix":
				fix = true
			case "--deep":
				deep = true
			}
		}
		err = commands.Doctor(fix, deep)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", args[0])
		commands.Help()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// setenvFn is the signature of os.Setenv; abstracted so applyGlobalFlags can
// be unit-tested without mutating process state.
type setenvFn func(key, value string) error

// applyGlobalFlags scans args for the top-level flags that control
// authentication, offline mode, and mirrors, applies them via setEnv, and
// returns a new args slice with those flags removed so subcommand parsers
// never see them.
//
// Supported flags (equivalent env var in parentheses):
//
//	--offline                               (ISTIOENV_OFFLINE=1)
//	--github-token <token>  --github-token=<token>    (ISTIOENV_GITHUB_TOKEN)
//	--api-mirror <url>      --api-mirror=<url>        (ISTIOENV_API_MIRROR)
//	--download-mirror <url> --download-mirror=<url>   (ISTIOENV_DOWNLOAD_MIRROR)
//
// Flags may appear anywhere on the command line (before or after the
// subcommand). The explicit env var still wins if both are supplied; flags
// only *set* the env var when the user actually passed them.
//
// A `--` sentinel ends global-flag parsing: everything after it is passed
// through untouched (useful for `istioctl-env exec`).
func applyGlobalFlags(args []string, setEnv setenvFn) ([]string, error) {
	out := make([]string, 0, len(args))
	passthrough := false
	for i := 0; i < len(args); i++ {
		a := args[i]

		if passthrough {
			out = append(out, a)
			continue
		}
		if a == "--" {
			passthrough = true
			out = append(out, a)
			continue
		}

		// Boolean flag.
		if a == "--offline" {
			if err := setEnv("ISTIOENV_OFFLINE", "1"); err != nil {
				return nil, fmt.Errorf("failed to set ISTIOENV_OFFLINE: %w", err)
			}
			continue
		}

		// String flags: handle both --flag=value and --flag value.
		if val, ok := takeFlagValue(a, "--github-token", &i, args); ok {
			if err := setEnv("ISTIOENV_GITHUB_TOKEN", val); err != nil {
				return nil, err
			}
			continue
		}
		if val, ok := takeFlagValue(a, "--api-mirror", &i, args); ok {
			if err := setEnv("ISTIOENV_API_MIRROR", val); err != nil {
				return nil, err
			}
			continue
		}
		if val, ok := takeFlagValue(a, "--download-mirror", &i, args); ok {
			if err := setEnv("ISTIOENV_DOWNLOAD_MIRROR", val); err != nil {
				return nil, err
			}
			continue
		}

		// Reject bare "--flag" with no value for the string flags, otherwise
		// we would silently consume the subcommand name as a flag value.
		if a == "--github-token" || a == "--api-mirror" || a == "--download-mirror" {
			return nil, fmt.Errorf("flag %s requires a value", a)
		}

		out = append(out, a)
	}
	return out, nil
}

// takeFlagValue reports whether arg matches the given flag name and, if so,
// returns its value. It handles both forms:
//
//	--flag=value          → value is parsed inline; i is unchanged.
//	--flag value          → value is read from args[i+1] and i is advanced.
//
// It returns ok=false when arg does not match flagName at all. If arg matches
// but no value follows, ok is also false; the caller is responsible for
// surfacing the error (see applyGlobalFlags).
func takeFlagValue(arg, flagName string, i *int, args []string) (string, bool) {
	if arg == flagName {
		if *i+1 >= len(args) {
			return "", false
		}
		*i++
		return args[*i], true
	}
	prefix := flagName + "="
	if strings.HasPrefix(arg, prefix) {
		return strings.TrimPrefix(arg, prefix), true
	}
	return "", false
}
