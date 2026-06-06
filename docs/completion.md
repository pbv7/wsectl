# Shell Completion

`wsectl` uses Cobra's generated shell completion support. Completion scripts are generated from the command tree in the binary, so they automatically include new commands, flags, and registered enum completions after `wsectl` is updated.

Supported shells:

```bash
wsectl completion bash
wsectl completion zsh
wsectl completion fish
wsectl completion powershell
```

## Bash

For the current shell:

```bash
source <(wsectl completion bash)
```

For future shells on Linux:

```bash
wsectl completion bash > ~/.local/share/bash-completion/completions/wsectl
```

System-wide locations vary by distribution, commonly:

```bash
wsectl completion bash | sudo tee /etc/bash_completion.d/wsectl >/dev/null
```

## Zsh

For the current shell:

```bash
source <(wsectl completion zsh)
```

To install persistently, write the generated file into a directory on `fpath`:

```bash
mkdir -p ~/.zsh/completions
wsectl completion zsh > ~/.zsh/completions/_wsectl
```

Then ensure this appears before `compinit` in `~/.zshrc`:

```zsh
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit
compinit
```

## Fish

```bash
mkdir -p ~/.config/fish/completions
wsectl completion fish > ~/.config/fish/completions/wsectl.fish
```

Fish loads the file automatically in new shells.

## PowerShell

For the current session:

```powershell
wsectl completion powershell | Out-String | Invoke-Expression
```

To install persistently:

```powershell
New-Item -ItemType Directory -Force (Split-Path $PROFILE)
wsectl completion powershell >> $PROFILE
```

## What Completes

Cobra automatically completes commands and flags. `wsectl` also registers completions for common constrained values:

- `--output`: `auto`, `json`, `yaml`, `table`, `ndjson`, `raw`
- `--profile`: configured profile names
- `profiles add --auth-type`: `oauth2`, `admin_token`
- `api call ACTION`: registered read-only Worksection actions
- `api schema ACTION`: registered Worksection actions
- project and task statuses
- tag type/access values
- cost timer booleans
- comma-separated `--extra` values on project and task commands

## Regeneration

Completion files are generated artifacts. Re-run the relevant command after upgrading `wsectl`:

```bash
wsectl completion zsh > ~/.zsh/completions/_wsectl
```
