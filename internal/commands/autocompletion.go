package commands

import (
	"fmt"

	"github.com/user/istioctl-env/internal/shim"
)

// Autocompletion prints the autocompletion script for the requested shell.
//
// shell may be one of "bash", "zsh", "fish", or "" to auto-detect from $SHELL.
// When auto-detection fails, a warning is printed to stderr and bash is used.
func Autocompletion(shell string) error {
	resolvedShell, err := resolveShell(shell)
	if err != nil {
		return err
	}
	switch resolvedShell {
	case shim.ShellZsh:
		fmt.Print(AutocompletionZsh())
	case shim.ShellFish:
		fmt.Print(AutocompletionFish())
	default:
		fmt.Print(AutocompletionBash())
	}
	return nil
}

// AutocompletionBash returns the bash completion script. The script shape and
// subcommand list are referenced by the bats integration tests and must stay
// stable.
func AutocompletionBash() string {
	return `_istioctl_env_completions() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="help list list-remote init install uninstall prune shell local global latest which exec resolve status upgrade doctor version"

    if [[ ${COMP_CWORD} -eq 1 ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi

    case "${prev}" in
        install|uninstall|shell|local|global|exec)
            # Suggest installed versions for these commands
            local versions=$(istioctl-env list | awk '{print $1}')
            COMPREPLY=( $(compgen -W "${versions}" -- "${cur}") )
            return 0
            ;;
    esac
}
complete -F _istioctl_env_completions istioctl-env
`
}

// AutocompletionZsh returns a zsh completion script. It uses the standard
// `#compdef` header plus `_arguments`-driven subcommand dispatch with
// per-subcommand descriptions. For `install` it offers remote versions read
// from the on-disk release cache (via `istioctl-env list-remote --cached`) so
// completion stays fast and works offline; for `uninstall`, `shell`, `local`,
// `global`, and `exec` it offers installed versions via `istioctl-env list`.
func AutocompletionZsh() string {
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
                'resolve:Resolve the active version expression to a concrete version'
                'status:Show current istioctl-env environment status'
                'upgrade:Upgrade istioctl-env to the latest version'
                'doctor:Diagnose the istioctl-env environment'
                'autocompletion:Print the shell completion script'
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
                        '1:version:__istioctl_env_installed_versions' \
                        '*::command: _normal'
                    ;;
                resolve)
                    _arguments \
                        '--install[Install the best matching remote version if nothing installed matches]' \
                        '(-s --silent)'{-s,--silent}'[Suppress installer progress output]' \
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
                init|autocompletion)
                    _arguments \
                        '--shell[Target shell for the generated snippet]:shell:(bash zsh fish)' \
                        '(-h --help)'{-h,--help}'[Show help]'
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

// AutocompletionFish returns a fish completion script. It wires up top-level
// subcommand suggestions and, for the subcommands that take a version
// argument, dynamically expands to the installed versions.
func AutocompletionFish() string {
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
complete -c istioctl-env -n '__fish_use_subcommand' -a 'resolve' -d 'Resolve the active version expression to a concrete version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'status' -d 'Show current istioctl-env environment status'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'upgrade' -d 'Upgrade istioctl-env to the latest version'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'doctor' -d 'Diagnose the istioctl-env environment'
complete -c istioctl-env -n '__fish_use_subcommand' -a 'version' -d 'Print the version of istioctl-env'

complete -c istioctl-env -n '__fish_seen_subcommand_from install' -a '(istioctl-env list-remote --cached 2>/dev/null)' -d 'Remote version'
complete -c istioctl-env -n '__fish_seen_subcommand_from uninstall shell local global exec' -a '(istioctl-env list | awk \'{print $1}\')' -d 'Installed version'

complete -c istioctl-env -n '__fish_seen_subcommand_from install' -s s -l silent -d 'Suppress progress and verification output'
complete -c istioctl-env -n '__fish_seen_subcommand_from list-remote latest' -l prerelease -d 'Include pre-release versions'
complete -c istioctl-env -n '__fish_seen_subcommand_from list-remote latest' -l cached -d 'Use on-disk cache only (no network)'
complete -c istioctl-env -n '__fish_seen_subcommand_from init autocompletion' -l shell -d 'Target shell' -xa 'bash zsh fish'
complete -c istioctl-env -n '__fish_seen_subcommand_from resolve' -l install -d 'Install the best matching remote version if nothing installed matches'
complete -c istioctl-env -n '__fish_seen_subcommand_from resolve' -s s -l silent -d 'Suppress installer progress output'
complete -c istioctl-env -n '__fish_seen_subcommand_from doctor' -l fix -d 'Attempt to automatically repair issues'
complete -c istioctl-env -n '__fish_seen_subcommand_from doctor' -l deep -d 'Also re-verify installed binaries against GitHub checksums'
`
}

// AutocompletionHelp prints help for the autocompletion command.
func AutocompletionHelp() {
	fmt.Println(`Usage: istioctl-env autocompletion [--shell <bash|zsh|fish>]

Flags:
  --shell <name>   Target shell for the completion script. Defaults to
                   auto-detection via $SHELL and falls back to bash.
  -h, --help       Show this help and exit.

To enable autocompletion, add one of the following to your shell profile:
  # bash (~/.bashrc)
  source <(istioctl-env autocompletion)

  # zsh (~/.zshrc, with fpath configured or using source):
  source <(istioctl-env autocompletion --shell zsh)

  # fish (~/.config/fish/completions/istioctl-env.fish):
  istioctl-env autocompletion --shell fish > ~/.config/fish/completions/istioctl-env.fish`)
}
