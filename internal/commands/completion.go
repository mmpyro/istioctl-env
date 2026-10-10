package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/istioctl-env/internal/shim"
)

// shellPowerShell is a completion-only shell: init does not support it.
const shellPowerShell = "powershell"

// Completion prints the completion script for the requested shell.
//
// shell may be one of "bash", "zsh", "fish", "powershell" (alias "pwsh"), or
// "" to auto-detect from $SHELL. When auto-detection fails, a warning is
// printed to stderr and bash is used.
func Completion(shell string) error {
	resolvedShell, err := resolveCompletionShell(shell)
	if err != nil {
		return err
	}
	switch resolvedShell {
	case shim.ShellZsh:
		fmt.Print(CompletionZsh())
	case shim.ShellFish:
		fmt.Print(CompletionFish())
	case shellPowerShell:
		fmt.Print(CompletionPowerShell())
	default:
		fmt.Print(CompletionBash())
	}
	return nil
}

// resolveCompletionShell extends resolveShell with PowerShell, which only
// completion supports.
func resolveCompletionShell(shell string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "powershell", "pwsh":
		return shellPowerShell, nil
	case "":
		switch filepath.Base(os.Getenv("SHELL")) {
		case "pwsh", "powershell":
			return shellPowerShell, nil
		}
		return resolveShell("")
	}
	if _, err := shim.NormalizeShell(shell); err != nil {
		return "", fmt.Errorf("unknown shell %q: valid choices are bash, zsh, fish, powershell", shell)
	}
	return resolveShell(shell)
}

// CompletionBash returns the bash completion script. The script shape and
// subcommand list are referenced by the bats integration tests and must stay
// stable.
func CompletionBash() string {
	return `_istioctl_env_completions() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="help list list-remote init install uninstall prune shell local global latest which exec resolve status upgrade doctor completion version"

    if [[ ${COMP_CWORD} -eq 1 ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi

    case "${prev}" in
        install|resolve)
            # Suggest remote versions from the on-disk release cache (no network)
            local versions=$(istioctl-env list-remote --cached 2>/dev/null)
            COMPREPLY=( $(compgen -W "${versions}" -- "${cur}") )
            return 0
            ;;
        uninstall|shell|local|global|exec)
            # Suggest installed versions for these commands
            local versions=$(istioctl-env list | awk '{print $1}')
            COMPREPLY=( $(compgen -W "${versions}" -- "${cur}") )
            return 0
            ;;
        completion|--shell)
            COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- "${cur}") )
            return 0
            ;;
    esac
}
complete -F _istioctl_env_completions istioctl-env
`
}

// CompletionZsh returns a zsh completion script. It uses the standard
// `#compdef` header plus `_arguments`-driven subcommand dispatch with
// per-subcommand descriptions. For `install` and `resolve` it offers remote
// versions read from the on-disk release cache (via `istioctl-env list-remote
// --cached`) so completion stays fast and works offline; for `uninstall`,
// `shell`, `local`, `global`, and `exec` it offers installed versions via
// `istioctl-env list`.
func CompletionZsh() string {
	return `#compdef istioctl-env

_istioctl-env() {
    local curcontext="$curcontext" state line
    typeset -A opt_args

    _arguments -C \
        '1: :->command' \
        '*:: :->args'

    case $state in
        command)
            local -a subcommands
            subcommands=(
                'help:Display help and all available commands'
                'version:Print the version of istioctl-env'
                'init:Initialize istioctl-env setup'
                'list:List installed istioctl versions'
                'list-remote:List available istioctl versions from GitHub'
                'latest:Print the latest available version'
                'install:Install a specific version (or latest)'
                'uninstall:Uninstall a specific version'
                'prune:Remove unused installed versions'
                'shell:Set or show the shell version'
                'local:Set or show the local version'
                'global:Set or show the global version'
                'which:Print the path to the active istioctl binary'
                'exec:Run a command using a specific istioctl version'
                'resolve:Resolve a version expression to a concrete version'
                'status:Show current istioctl-env environment status'
                'upgrade:Upgrade istioctl-env to the latest version'
                'doctor:Diagnose the istioctl-env environment'
                'completion:Print the shell completion script'
            )
            _describe -t commands 'istioctl-env command' subcommands
            ;;
        args)
            case $words[1] in
                install)
                    _arguments \
                        '(-s --silent)'{-s,--silent}'[Suppress progress and verification output]' \
                        '(-h --help)'{-h,--help}'[Show help for install]' \
                        '1:version:__istioctl_env_remote_versions'
                    ;;
                uninstall|shell|local|global)
                    _arguments '1:version:__istioctl_env_installed_versions'
                    ;;
                exec)
                    _arguments \
                        '(--no-auto)--auto[Install the best matching remote version if needed]' \
                        '(--auto)--no-auto[Never install missing versions]' \
                        '1:version:__istioctl_env_installed_versions' \
                        '*::command: _normal'
                    ;;
                resolve)
                    _arguments \
                        '--install[Install the best matching remote version if nothing installed matches]' \
                        '(-s --silent)'{-s,--silent}'[Suppress installer progress output]' \
                        '(-h --help)'{-h,--help}'[Show help]' \
                        '1:version:__istioctl_env_remote_versions'
                    ;;
                prune)
                    _arguments \
                        '--keep-last[Keep the N most recent versions]:count:' \
                        '--older-than[Only prune versions older than DUR]:duration:' \
                        '--dry-run[Show what would be removed]' \
                        '--yes[Actually remove versions]' \
                        '(-h --help)'{-h,--help}'[Show help]'
                    ;;
                doctor)
                    _arguments \
                        '--fix[Attempt to automatically repair issues]' \
                        '--deep[Also re-verify installed binaries against GitHub checksums]' \
                        '(-h --help)'{-h,--help}'[Show help]'
                    ;;
                list-remote|latest)
                    _arguments \
                        '--prerelease[Include pre-release versions]' \
                        '--cached[Use the on-disk cache only (no network)]' \
                        '(-h --help)'{-h,--help}'[Show help]'
                    ;;
                init)
                    _arguments \
                        '--shell[Target shell for the generated snippet]:shell:(bash zsh fish)' \
                        '(-h --help)'{-h,--help}'[Show help]'
                    ;;
                completion)
                    _arguments \
                        '--shell[Target shell for the completion script]:shell:(bash zsh fish powershell)' \
                        '(-h --help)'{-h,--help}'[Show help]' \
                        '1:shell:(bash zsh fish powershell)'
                    ;;
            esac
            ;;
    esac
}

__istioctl_env_installed_versions() {
    local -a vs
    vs=(${(f)"$(istioctl-env list 2>/dev/null | awk '{print $1}')"})
    _describe -t versions 'installed version' vs
}

# Remote versions come from the on-disk release cache, which is populated by
# every call to list-remote / latest / install.  The --cached flag guarantees
# no network call is made, so completion stays fast even on the first TAB of a
# cold shell.
__istioctl_env_remote_versions() {
    local -a vs
    vs=(${(f)"$(istioctl-env list-remote --cached 2>/dev/null)"})
    _describe -t versions 'remote version' vs
}

compdef _istioctl-env istioctl-env
`
}

// CompletionFish returns a fish completion script. It wires up top-level
// subcommand suggestions and, for the subcommands that take a version
// argument, dynamically expands to the installed or cached remote versions.
func CompletionFish() string {
	return `# fish completion for istioctl-env

complete -c istioctl-env -f

complete -c istioctl-env -n '__fish_use_subcommand' -a 'help' -d 'Display help and all available commands'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'list' -d 'List installed istioctl versions'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'list-remote' -d 'List available istioctl versions from GitHub'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'init' -d 'Initialize istioctl-env setup'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'install' -d 'Install a specific version (or latest)'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'uninstall' -d 'Uninstall a specific version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'prune' -d 'Remove unused installed versions'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'shell' -d 'Set or show the shell version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'local' -d 'Set or show the local version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'global' -d 'Set or show the global version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'latest' -d 'Print the latest available version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'which' -d 'Print the path to the active istioctl binary'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'exec' -d 'Run a command using a specific istioctl version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'resolve' -d 'Resolve a version expression to a concrete version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'status' -d 'Show current istioctl-env environment status'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'upgrade' -d 'Upgrade istioctl-env to the latest version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'doctor' -d 'Diagnose the istioctl-env environment'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'completion' -d 'Print the shell completion script'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'version' -d 'Print the version of istioctl-env'

complete -c istioctl-env -n '__fish_seen_subcommand_from install resolve' -a '(istioctl-env list-remote --cached 2>/dev/null)' -d 'Remote version'
complete -c istioctl-env -n '__fish_seen_subcommand_from uninstall shell local global exec' -a '(istioctl-env list | awk \'{print $1}\')' -d 'Installed version'
complete -c istioctl-env -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish powershell' -d 'Shell'

complete -c istioctl-env -n '__fish_seen_subcommand_from install' -s s -l silent -d 'Suppress progress and verification output'
complete -c istioctl-env -n '__fish_seen_subcommand_from list-remote latest' -l prerelease -d 'Include pre-release versions'
complete -c istioctl-env -n '__fish_seen_subcommand_from list-remote latest' -l cached -d 'Use on-disk cache only (no network)'
complete -c istioctl-env -n '__fish_seen_subcommand_from init' -l shell -d 'Target shell' -xa 'bash zsh fish'
complete -c istioctl-env -n '__fish_seen_subcommand_from completion' -l shell -d 'Target shell' -xa 'bash zsh fish powershell'
complete -c istioctl-env -n '__fish_seen_subcommand_from resolve' -l install -d 'Install the best matching remote version if nothing installed matches'
complete -c istioctl-env -n '__fish_seen_subcommand_from resolve' -s s -l silent -d 'Suppress installer progress output'
complete -c istioctl-env -n '__fish_seen_subcommand_from exec' -l auto -d 'Install the best matching remote version if needed'
complete -c istioctl-env -n '__fish_seen_subcommand_from exec' -l no-auto -d 'Never install missing versions'
complete -c istioctl-env -n '__fish_seen_subcommand_from prune' -l keep-last -x -d 'Keep the N most recent versions'
complete -c istioctl-env -n '__fish_seen_subcommand_from prune' -l older-than -x -d 'Only prune versions older than DUR'
complete -c istioctl-env -n '__fish_seen_subcommand_from prune' -l dry-run -d 'Show what would be removed'
complete -c istioctl-env -n '__fish_seen_subcommand_from prune' -l yes -d 'Actually remove versions'
complete -c istioctl-env -n '__fish_seen_subcommand_from doctor' -l fix -d 'Attempt to automatically repair issues'
complete -c istioctl-env -n '__fish_seen_subcommand_from doctor' -l deep -d 'Also re-verify installed binaries against GitHub checksums'
`
}

// CompletionPowerShell returns a PowerShell completion script that registers
// a native argument completer. Works with Windows PowerShell 5.1 and
// PowerShell 7+ (pwsh).
//
// Go raw-string literals cannot contain backticks, and PowerShell uses the
// backtick as its escape character, so the template uses a {BT} placeholder.
func CompletionPowerShell() string {
	return strings.ReplaceAll(powershellCompletionTemplate, "{BT}", "`")
}

const powershellCompletionTemplate = `# istioctl-env PowerShell completion
Register-ArgumentCompleter -Native -CommandName istioctl-env -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $subcommands = @(
        'help','list','list-remote','init','install','uninstall','prune','shell','local','global',
        'latest','which','exec','resolve','status','upgrade','doctor','completion','version'
    )
    $shells = @('bash','zsh','fish','powershell')

    # Tokenise the command line, dropping the program name itself.
    $tokens = @($commandAst.CommandElements | ForEach-Object { $_.ToString() })
    if ($tokens.Count -gt 0) { $tokens = $tokens[1..($tokens.Count - 1)] }

    # Decide whether the user is completing the subcommand slot or an argument.
    $completingSubcommand = $tokens.Count -eq 0 -or
        ($tokens.Count -eq 1 -and $wordToComplete -eq $tokens[0])

    if ($completingSubcommand) {
        $subcommands |
            Where-Object { $_ -like "$wordToComplete*" } |
            ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
        return
    }

    $sub = $tokens[0]
    $candidates = @()
    switch ($sub) {
        { @('install','resolve') -contains $_ } {
            $candidates = (istioctl-env list-remote --cached 2>$null) -split "{BT}r?{BT}n" | Where-Object { $_ }
        }
        { @('uninstall','shell','local','global','exec') -contains $_ } {
            $candidates = (istioctl-env list 2>$null) -split "{BT}r?{BT}n" | Where-Object { $_ }
        }
        'completion' { $candidates = $shells }
        default      { $candidates = @() }
    }

    $candidates |
        Where-Object { $_ -like "$wordToComplete*" } |
        ForEach-Object { [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_) }
}
`

// CompletionHelp prints help for the completion command.
func CompletionHelp() {
	fmt.Println(`Usage: istioctl-env completion [<shell>] [--shell <shell>]

Print a shell completion script.  <shell> is one of bash, zsh, fish or
powershell (alias: pwsh).  Without a shell, it is detected from $SHELL,
falling back to bash with a warning on stderr.

Flags:
  --shell <name>   Same as the positional <shell> argument.
  -h, --help       Show this help and exit.

To enable completion, add one of the following to your shell profile:
  # bash (~/.bashrc)
  source <(istioctl-env completion bash)

  # zsh (~/.zshrc, with compinit loaded):
  source <(istioctl-env completion zsh)

  # fish (~/.config/fish/completions/istioctl-env.fish):
  istioctl-env completion fish > ~/.config/fish/completions/istioctl-env.fish

  # PowerShell ($PROFILE):
  istioctl-env completion powershell | Out-String | Invoke-Expression

'istioctl-env autocompletion' is a deprecated alias for this command.`)
}
