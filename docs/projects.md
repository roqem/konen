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
non-empty command is passed to that shell with `-lc`; `hold = true` asks Kitty
to open a shell after the command exits.

Use the project from its directory or by name:

```console
konen dev
konen dev my-app
konen dev my-app --dry-run
```

Inside Kitty, Konen uses remote control to add tabs to the same OS window and
then focuses the first created tab. The invoking tab remains open. This requires
`allow_remote_control yes` in `kitty.conf`. Outside Kitty, Konen renders a
temporary native session file and opens a new Kitty window.

## Manifest

The guided flow writes `projects/NAME.toml` in the Konen state:

```toml
version = 1
path = "~/Documents/Projects/my-app"

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

Edit it through the guided form with `konen project edit NAME`. `show`, `list`
and `--dry-run` are non-mutating inspection commands.

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
