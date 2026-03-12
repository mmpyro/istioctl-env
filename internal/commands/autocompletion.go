package commands

import (
	"fmt"
)

// Autocompletion prints the bash autocompletion script.
func Autocompletion() error {
	script := `_istioctl_env_completions() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="help list list-remote init install uninstall shell local global latest which exec status upgrade version"

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
	fmt.Print(script)
	return nil
}

// AutocompletionHelp prints help for the autocompletion command.
func AutocompletionHelp() {
	fmt.Println(`Usage: istioctl-env autocompletion

Prints the bash autocompletion script.
To enable autocompletion, add the following line to your ~/.bashrc:
  source <(istioctl-env autocompletion)`)
}
