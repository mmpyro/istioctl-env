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

	args := os.Args[1:]

	if len(args) == 0 {
		commands.Help()
		os.Exit(0)
	}

	var err error

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

	case "resolve":
		install := false
		silent := false
		for _, arg := range args[1:] {
			switch arg {
			case "-h", "--help":
				commands.ResolveHelp()
				os.Exit(0)
			case "--install":
				install = true
			case "-s", "--silent":
				silent = true
			}
		}
		err = commands.Resolve(install, silent)

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
