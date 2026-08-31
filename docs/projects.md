# Projects and Kitty sessions

Konen keeps workspace sessions in the versionable machine state instead of
adding Kitty or editor files to every source repository.

## Workflow

From a project directory, run:

```console
konen project add
```

The guided form asks for a short name, the project directory, an optional shell
and one or more tabs. An empty command opens a login shell in the project. A
non-empty command is passed to an interactive login shell with `-lic`, so the
workspace sees the environment configured in `.zshrc`; `hold = true` asks Kitty
to open a shell after the command exits.

Use the project from its directory or by name:

```console
konen projects
konen dev
konen dev my-app
konen my-app
konen dev my-app --dry-run
```

`konen NAME` is the short form of `konen dev NAME` for a registered project;
it never scans arbitrary folders or registers one implicitly. `konen projects`
is the canonical list command. `konen project list` remains available as a
compatibility alias beside the singular project actions.

Inside Kitty, Konen uses remote control to add tabs to the same OS window and
then focuses the first created tab. The invoking tab remains open by default.
This requires `allow_remote_control yes` in `kitty.conf`. Outside Kitty, Konen
renders a temporary native session file and opens a new Kitty window.

## Manifest

The guided flow writes `projects/NAME.toml` in the Konen state:

```toml
version = 1
path = "~/Documents/Projects/my-app"
keep_invoking_tab = false

[[tabs]]
title = "Neovim"
command = "nvim ."

[[tabs]]
title = "Docker"
command = "docker compose --profile=mongo up -d && docker compose exec web sh"

[[tabs]]
title = "Claude"
command = "claude"

[[tabs]]
title = "Terminal"
command = "git status"
hold = true
```

`keep_invoking_tab` defaults to `true` for existing manifests. Setting it to
`false` closes only the Kitty terminal that invoked `konen dev` after all tabs
have opened and the first has received focus; if it is the only terminal in its
tab, Kitty closes that tab as well.

Edit it through the guided form with `konen project edit NAME`. `show`, `list`
and `--dry-run` are non-mutating inspection commands. The list and dry-run
output also report whether each local approval is valid or needs review.

## Trust

Project manifests contain executable commands. Approval is therefore local and
bound to the exact SHA-256 digest of each manifest; it is not committed with the
state. The add/edit wizard approves what it has just written. Changes made by a
Git pull or another editor invalidate that approval:

```console
konen project show my-app
konen project trust my-app
```

Konen will not launch any tab until the current manifest has been approved.
Moving the state repository changes each manifest's absolute identity and
therefore requires a new local approval even when its contents are unchanged.
