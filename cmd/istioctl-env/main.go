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
		err = commands.Init()

	case "list":
		err = commands.List()

	case "list-remote":
		includePrerelease := false
		for _, arg := range args[1:] {
			switch arg {
			case "-h", "--help":
				commands.ListRemoteHelp()
				os.Exit(0)
			case "--prerelease":
				includePrerelease = true
			}
		}
		err = commands.ListRemote(includePrerelease)

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
		for _, arg := range args[1:] {
			if arg == "-h" || arg == "--help" {
				commands.AutocompletionHelp()
				os.Exit(0)
			}
		}
		err = commands.Autocompletion()

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
