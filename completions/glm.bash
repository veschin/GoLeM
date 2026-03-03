# Bash completion for glm
# Source: source /path/to/glm.bash or copy to /usr/share/bash-completion/completions/glm

_glm() {
    local cur prev words cword
    _init_completion || return

    local commands="session run start status result log list clean kill chain update doctor config _install _uninstall version help"
    local flags="-d -t -m --model --opus --sonnet --haiku --unsafe --mode --json"
    local config_keys="model opus_model sonnet_model haiku_model permission_mode max_parallel debug"
    local status_values="queued running done failed cancelled permission_error"
    local modes="bypassPermissions acceptEdits plan"

    # Determine command position
    local cmd=""
    local i=1
    while [[ $i -lt $cword ]]; do
        if [[ ${words[$i]} != -* ]]; then
            cmd=${words[$i]}
            break
        fi
        ((i++))
    done

    case $cmd in
        session|run|start|chain)
            # These accept common flags
            case $prev in
                -d)
                    _filedir -d
                    return
                    ;;
                -t)
                    return
                    ;;
                -m|--model|--opus|--sonnet|--haiku)
                    COMPREPLY=($(compgen -W "glm-4.7 glm-4 glm-4-flash" -- "$cur"))
                    return
                    ;;
                --mode)
                    COMPREPLY=($(compgen -W "$modes" -- "$cur"))
                    return
                    ;;
            esac
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "$flags" -- "$cur"))
                return
            fi
            ;;
        status|result|log|kill)
            # These take a JOB_ID
            if [[ $cur != -* ]]; then
                # Try to complete job IDs from subagents directory
                local jobs_dir="$HOME/.claude/subagents"
                if [[ -d "$jobs_dir" ]]; then
                    local jobs=$(find "$jobs_dir" -maxdepth 2 -name "job-*" -type d 2>/dev/null | sed 's|.*/||' | head -50)
                    COMPREPLY=($(compgen -W "$jobs" -- "$cur"))
                fi
            fi
            return
            ;;
        list)
            case $prev in
                --status)
                    COMPREPLY=($(compgen -W "$status_values" -- "$cur"))
                    return
                    ;;
                --since)
                    return
                    ;;
            esac
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "--status --since --json" -- "$cur"))
            fi
            return
            ;;
        clean)
            case $prev in
                --days)
                    return
                    ;;
            esac
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "--days" -- "$cur"))
            fi
            return
            ;;
        config)
            case $prev in
                config)
                    COMPREPLY=($(compgen -W "show set" -- "$cur"))
                    return
                    ;;
                set)
                    COMPREPLY=($(compgen -W "$config_keys" -- "$cur"))
                    return
                    ;;
            esac
            return
            ;;
        doctor)
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "--json" -- "$cur"))
            fi
            return
            ;;
        "")
            # No command yet
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "--version --help -v -h" -- "$cur"))
            else
                COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            fi
            return
            ;;
    esac
}

complete -F _glm glm
