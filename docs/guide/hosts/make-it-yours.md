# Make it yours

aayu should feel good to use. Run it on its own in a repository and it tells you where the work stands and what to
do next. This page shows how to tune the look, turn it down to plain words, and teach your shell to finish your
commands.

## The home screen and your next step

```sh
aayu
```

With no command, `aayu` shows the home screen: where the repository is on the journey (Set up, Shape, Ready, Build,
Prove, Ship, Release, Learn), its adoption level, the stories that are Ready or in progress, the pull requests
waiting for a person, failing floors, measurements that are due, and the one command worth running next.

```sh
aayu next
```

`aayu next` prints just that command on its first line, then why. Scripts can take the first line. When the next step
is a person's (shipping, releasing, accepting a Decision), `next` says so, and an agent asking `next` gets the best
step it may take itself instead.

> **Tip:** every error ends with a `next:` line holding the exact command that fixes it. With `--json`, the same
> command is in `error.next`.

## The seats speak

Each command belongs to one seat, and that seat says a line before the facts and signs off after them. The lines come
from the seat's charter in `seats/<seat>/CHARTER.md`. The facts themselves stay plain, and the seat's lines go to the
error stream, so `aayu status > status.md` holds only the result.

## Themes

Set a theme in your own config, `~/.config/aayudha/config.toml`:

```toml
[ui]
theme = "saffron"
```

| Theme | What it looks like |
|---|---|
| `auto` (the default) | Your terminal's own 16 colours, so it follows your light or dark terminal theme |
| `saffron` | Warm true colour, made for dark terminals |
| `contrast` | Bright, bold colours for low vision |
| `mono` | No colour at all: bold and underline only |

Colours only ever repeat what the words say. A check is always written `PASS`, `WARN`, `FAIL`, or `N/A`, with or
without its colour and symbol.

## Plain mode

Plain mode is words only: no colour, no spinners, no emoji, no box drawing. It suits screen readers, braille displays,
and anyone who prefers a quiet terminal. Turn it on in any of these ways:

| How | Scope |
|---|---|
| `aayu --plain …` | One command |
| `AAYU_PLAIN=1` in your environment | Every command in that shell |
| `plain = true` under `[ui]` in `~/.config/aayudha/config.toml` | Always |

aayu also goes plain on its own when `TERM=dumb`, and drops colour when `NO_COLOR` is set.

Machines never see any of this. Output for `--json`, the MCP server, agents (`AAYU_AGENT=1`), CI, and anything that is
not a terminal is the same stable text it has always been, and exit codes never change.

## Celebrations

A few moments deserve a cheer: your first Ready story, your first proven ship, reaching Level 1, and every release. On
a terminal you get a short sparkle; in plain mode, one line that starts with `Milestone:`. Each first happens once per
checkout.

## The guided first run

```sh
aayu tour
```

The tour checks your machine, asks which agent you work in, then runs `aayu init` and `aayu install` for you. Each
of them still shows you what it changes and asks before it uses your admin access. At the end, your repository is at
Level 1.

## Shell completions

Completions come from the same registry as the commands, so every verb, flag, and choice (`grill story`,
`release --to rolling`, `install --host codex`) completes.

| Shell | Add this to your shell's startup file |
|---|---|
| bash | `source <(aayu completion bash)` in `~/.bashrc` |
| zsh | `source <(aayu completion zsh)` in `~/.zshrc` |
| fish | `aayu completion fish \| source` in `~/.config/fish/config.fish` |
| PowerShell | `aayu completion powershell \| Out-String \| Invoke-Expression` in your `$PROFILE` |

The scripts complete the name `aayu`. If you call the binary `aayudha`, add `alias aayu=aayudha` as well.
