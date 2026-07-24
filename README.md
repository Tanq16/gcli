<div align="center">
  <img src=".github/assets/logo.svg" alt="gcli Logo" width="200">
  <h1>gcli</h1>

  <a href="https://github.com/tanq16/gcli/actions/workflows/release.yaml"><img alt="Build Workflow" src="https://github.com/tanq16/gcli/actions/workflows/release.yaml/badge.svg"></a>&nbsp;<a href="https://github.com/tanq16/gcli/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/tanq16/gcli"></a><br><br>
  <a href="#capabilities">Capabilities</a> &bull; <a href="#installation">Installation</a> &bull; <a href="#authentication">Authentication</a> &bull; <a href="#usage">Usage</a> &bull; <a href="#output-modes--scripting">Output Modes</a> &bull; <a href="#tips-and-notes">Tips & Notes</a>
</div>

---

gcli is a fully command-line tool for **Google Drive** (the primary product) and **Gmail** (a secondary, general-aid). Drive is treated as optimized storage: upload, download, `cat`, mirroring `sync`, and sharing, all with a Linux-style command surface, a shared concurrent transfer engine, and recoverable deletes. Gmail covers everyday email automation, send, reply, forward, drafts, and thread state, and nothing more. Every command speaks three output tiers so the same tool works for a human at a terminal, a `--debug` session, and a fully non-interactive `--for-ai` agent.

## Capabilities

| Service | Commands | Description |
|---------|----------|-------------|
| Auth | `login`, `login --setup`, `logout`, `whoami` | BYO OAuth (PKCE, paste-the-redirect-URL), guided setup wizard, revoke, and account/storage status |
| Drive | `ls` (`list`), `info`, `search` | Browse folders, inspect one item, search by name/content/type/time/size |
| Drive | `up` (`upload`), `dl` (`download`), `cat` | Concurrent, resumable transfers; stream remote bytes to stdout |
| Drive | `sync` | Destructive-mirror folder↔folder (or file↔file) with recoverable deletes, `--reverse`, `--backup`, `--dry-run` |
| Drive | `mkdir`, `mv` (`move`), `rm` (`remove`/`delete`), `purge` | Create, move+rename, trash (recoverable), and permanently delete |
| Drive | `trash list/restore/empty` | List, restore by name, and empty Drive trash |
| Drive | `share`, `unshare` | Grant/revoke by email, anyone-with-link, expiring links; list current shares |
| Mail | `list`, `search`, `get` (`view`) | List/search threads; read a full thread (HTML rendered to text) |
| Mail | `send`, `reply`, `forward` | Compose with attachments and signatures; forward re-attaches originals |
| Mail | `mark <verb>` | `read`, `unread`, `star`, `unstar`, `archive`, `trash`, `spam` |
| Mail | `drafts` (`draft`) `list/get/edit/send/rm` | Manage Gmail drafts; create a draft with `--draft` on any compose command |

File revisions are exposed as flags on existing commands, not a separate group: `info --revisions` (list), `download --revision <id>` (fetch an old version), `upload --keep-revision` (pin the pushed head).

## Installation

### Binary

Download from [releases](https://github.com/tanq16/gcli/releases):

```bash
# Linux/macOS
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64; [ "$ARCH" = "aarch64" ] && ARCH=arm64
curl -sL https://github.com/tanq16/gcli/releases/latest/download/gcli-$(uname -s | tr '[:upper:]' '[:lower:]')-$ARCH -o gcli
chmod +x gcli
sudo mv gcli /usr/local/bin/
```

### go install

```bash
go install github.com/tanq16/gcli@latest
```

### Build from Source

```bash
git clone https://github.com/tanq16/gcli
cd gcli
make build
```

## Authentication

gcli uses a **bring-your-own OAuth client** (BYO). There is no shipped shared client: an embedded secret in open-source cannot stay confidential, and a single client would hit Google's 100-user unverified cap across all installs. Setup is a one-time Google Cloud task; the wizard asks whether you already have a Desktop-app client, and if not prints a short block (two API links + the console steps) before taking your Client ID and secret:

```bash
gcli login --setup     # guided wizard: enable APIs, consent screen, Desktop-app client
gcli login             # opens your browser (or prints the URL); you paste the redirect URL back
gcli whoami            # which account am I, and will a backup fit?
gcli logout            # revoke the refresh token with Google, then delete it locally
```

One `login` flow serves desktop and headless/SSH alike: gcli opens your browser when it can, otherwise prints the authorization URL. Either way the browser lands on a `http://127.0.0.1/?...` page that won't load — copy that address bar URL and paste it back (gcli reads the `code` straight out of it), so there is no local listener and no manual code-extraction.

Credentials live in `~/.config/gcli/` (`credentials.json` = the OAuth client, `token.json` = your grant). The auth scopes are fixed and narrow: full Drive plus Gmail modify.

### Documented-forever facts (do not re-litigate)

- **The "Google hasn't verified this app" warning is expected and safe.** It is your own personal client; at login click `Advanced → Continue`. You must add your own email under the consent screen's Test users, or login is blocked with "Access denied".
- **A personal (non-Workspace) account must use User type "External".** "Internal" only exists inside a Google Workspace organization. The app stays in "Testing" publishing status, which is fine for personal use.
- **"Testing" status revokes the refresh token roughly weekly.** If you would rather not re-run `gcli login` every ~7 days, click `Publish app` → In production on the consent screen (no verification/review needed for personal use); the 100-user unverified cap still applies but never matters for one person.
- **You must create a "Desktop app" OAuth client, not "Web application."** A Web client loads but fails at redirect time; gcli detects the wrong type and tells you.
- **API keys cannot access private Drive/Gmail data.** A key attributes quota to a project; it does not authenticate a user. Private-data calls with only a key return 401/403 unconditionally.
- **Device-code login is impossible for gcli's scopes.** Google's device-flow allowlist excludes full `drive` and all `gmail.*`, so there is no `--device-login`; the paste-the-redirect-URL flow covers headless hosts instead.
- **Service accounts / domain-wide delegation are not supported.** Service accounts have 0 GB owned-storage quota and cannot be a personal identity; DWD needs a Workspace org admin and is out of scope.

### Headless / non-interactive

All optional; set the pair to skip `credentials.json`, add the refresh token to skip `token.json`:

| Variable | Effect |
|----------|--------|
| `GCLI_CLIENT_ID` + `GCLI_CLIENT_SECRET` | OAuth client without `credentials.json` (both required together) |
| `GCLI_REFRESH_TOKEN` | Token without `token.json` — fully headless with the pair above |
| `GCLI_CONFIG_DIR` | Repoint the whole config directory (the multi-account escape hatch) |

Under `--for-ai`, `gcli login` prints the authorization URL (it never tries to open a browser) and reads the pasted redirect URL — or bare code — from piped stdin; for a fully unattended agent, prefer the env vars above.

## Usage

The command tree:

```
gcli                                    --debug | --for-ai   (root, mutually exclusive)
├── login    [--setup [--client-id --client-secret --overwrite]]
├── logout   [--local-only]
├── whoami
├── drive    --workers/-w 4 | --id | --shared/-s            (drive-persistent)
│   ├── list (ls)       [path]           --with-id --filter/-f
│   ├── info            <path>           --revisions
│   ├── search          <query>          --in --content --type/-t --ext/-e --created --modified
│   │                                    --size-min --size-max --limit/-n --sort --with-id
│   ├── mkdir           <path>           --parents/-p
│   ├── upload (up)     <local> [remote] --keep-revision
│   ├── download (dl)   <remote> [local] --format --revision
│   ├── cat             <remote>         --format
│   ├── sync            <local> <remote> --reverse/-R --backup --dry-run --ignore/-i --yes/-y
│   ├── move (mv)       <src> <dst>
│   ├── rm (remove, delete)  <path>...
│   ├── purge           <path>...        --yes/-y
│   ├── trash  list (ls) [--with-id] · restore <name> · empty [--yes/-y]
│   ├── share           <path>           --with --role/-r --anyone --expires
│   └── unshare         <path>           --with --anyone --all
└── mail
    ├── list                             --label --unread --limit/-n
    ├── search          <query>          --limit/-n
    ├── get (view)      <thread-id>      --with-quote
    ├── send                             -t -s -c --bcc -a -f --signature --draft
    ├── reply           <thread-id>      --all -a -f --signature --draft
    ├── forward         <thread-id>      -t -a -f --signature --draft
    ├── mark            <verb> <thread-id>   (read unread star unstar archive trash spam)
    └── drafts (draft)  list · get <id> · edit <id> -t -s -c --bcc -f -a · send <id> · rm <id>
```

### Drive-persistent flags

Declared once on `drive`, inherited by every subcommand:

| Flag | Short | Default | Meaning |
|------|-------|---------|---------|
| `--workers` | `-w` | `4` | Worker-pool size for upload/download/sync (clamped to at least 1) |
| `--id` | — | `false` | Treat every remote argument as a Drive ID; it is resolved to its path first. Supports `<id>/suffix` grafting |
| `--shared` | `-s` | `false` | Operate in the shared namespace (shared drives + shared-with-me) instead of My Drive |

### Drive — browse and inspect

```bash
gcli drive ls /backups                       # TYPE, NAME, SIZE, MODIFIED (no ID column)
gcli drive ls /backups --with-id             # add the ID column
gcli drive ls / -f report                     # client-side name filter
gcli drive -s ls                              # list shared drives + shared-with-me
gcli drive info /backups/tax-2025.pdf         # full metadata table + webViewLink
gcli drive info /backups/tax-2025.pdf --revisions

gcli drive search "tax" --in /backups --ext pdf
gcli drive search invoice --modified 30d --sort size --with-id
gcli drive search "quarterly plan" --content -t doc
```

`--created`/`--modified` accept a relative window (`45m`, `2h`, `7d`, `1w`) or an inclusive date range (`2026-01-01..2026-03-01`).

### Drive — transfer

```bash
gcli drive up ./tax-2025.pdf /backups         # creates, or overwrites an existing same-named file
gcli drive up ./photos /backups/photos -w 8   # adds/overwrites; never removes remote-only files
gcli drive up ./notes.md /backups --keep-revision

gcli drive dl /backups/tax-2025.pdf           # atomic .part + rename, MD5-verified
gcli drive dl /backups/photos ./restore -w 8
gcli drive dl "/notes/Meeting Notes" --format md
gcli drive dl /backups/tax-2025.pdf ./old.pdf --revision 0B9k...   # an earlier version

gcli drive cat /scripts/backup.sh | sh -n     # stream raw bytes to stdout
gcli drive cat "/notes/Plan" --format md --for-ai
```

`upload` is `cp -r`: it copies local → remote, overwriting a same-named file in place and creating what is missing, and **never deletes** remote-only files. There is no `--overwrite` flag; overwrite-by-name is the default.

### Drive — sync (mirror)

Arguments are **always `<local> <remote>`**, regardless of direction; `--reverse` flips the data flow, never the argument order. Sync is the only command that deletes, and deletes are recoverable on both sides.

```bash
gcli drive sync ~/photos /backups/photos              # push: mirror local → remote
gcli drive sync ~/photos /backups/photos --dry-run    # print the plan, change nothing
gcli drive sync ~/photos /backups/photos -R           # reverse (pull): mirror remote → local
gcli drive sync ~/notes.md /backups/notes.md          # file ↔ file
gcli drive sync ~/proj /backups/proj --backup -i "*.tmp,node_modules"
```

In human mode sync prompts before executing deletes; pass `--yes/-y` to skip the gate (required under `--for-ai` when the plan contains deletes). `--ignore/-i` globs filter **both** trees, so an ignored destination-only entry is never mirror-deleted.

### Drive — organize and delete

```bash
gcli drive mkdir -p /backups/2026/q3          # prints the new folder's ID
gcli drive mv /backups/a.pdf /archive         # move into a folder
gcli drive mv /backups/a.pdf /archive/b.pdf   # move + rename
gcli drive rm /backups/old.pdf /backups/tmp   # variadic; trash only (recoverable)
gcli drive purge /backups/old.pdf --yes       # permanent delete, bypasses trash

gcli drive trash list --with-id
gcli drive trash restore tax-2025.pdf
gcli drive trash empty --yes
```

### Drive — share

```bash
gcli drive share /backups/tax-2025.pdf --with alice@x.com --role writer
gcli drive share /photos/album --anyone
gcli drive share /docs/spec.md --with bob@x.com --expires 7d
gcli drive share /backups/tax-2025.pdf              # zero flags: list current shares + the link

gcli drive unshare /docs/spec.md --with bob@x.com
gcli drive unshare /photos/album --anyone
gcli drive unshare /docs/spec.md --all             # remove every non-owner grant
```

Every `share` invocation ends with the file's link. You never touch a raw permission ID; revocation is by email or type.

### Using Drive IDs

`--id` treats each remote argument as an ID resolved to its path, with `<id>/suffix` grafting:

```bash
gcli drive --id ls 1AbCdEf                     # list the folder that ID names
gcli drive --id mkdir 1AbC/reports             # create "reports" under folder 1AbC
gcli drive --id mv 1Src 1DstFolder/newname     # move + rename
gcli drive --id sync ~/photos 1AbC -R
```

When a path segment is ambiguous (duplicate sibling names), human mode prompts you to pick; `--for-ai` lists the candidate IDs and asks you to re-run with `--id`.

### Mail

```bash
gcli mail list                                 # recent INBOX threads
gcli mail list --unread --limit 5
gcli mail list --label SENT
gcli mail search "from:alice@example.com" -n 50
gcli mail get <thread-id>                       # full thread; HTML rendered to text
gcli mail get <thread-id> --with-quote

gcli mail send -t alice@x.com -s "Q3 numbers" -f ./q3.html
gcli mail send -t alice@x.com -s "Q3" -a ./chart.png -a ./data.csv
gcli mail reply <thread-id> --all
gcli mail forward <thread-id> -t bob@x.com      # re-attaches the original's own attachments

gcli mail mark read <thread-id>
gcli mail mark star <thread-id>
gcli mail mark archive <thread-id>
```

`send` requires `-t/--to` and `-s/--subject`; `forward` requires `-t/--to`. Mail tables always show the thread/draft ID — it is the only addressing handle.

### Drafts

Add `--draft` to `send`, `reply`, or `forward` to create a Gmail draft instead of sending (threading headers are preserved on reply/forward drafts):

```bash
gcli mail send -t alice@x.com -s "Q3 numbers" -f ./q3.html --draft
gcli mail drafts list
gcli mail drafts get r-8123...
gcli mail drafts edit r-8123... -s "Q3 numbers (final)" -a ./chart.png
gcli mail drafts send r-8123...
gcli mail drafts rm r-8123...
```

`drafts edit` overlays only the flags you set (`-s ""` clears the subject; omitted flags carry forward), and `-a` appends attachments.

### Old → new migration

The full canonical rename map; anything not listed is unchanged in spirit.

| Old | New |
|-----|-----|
| `gcli login --device-login` | removed (impossible for gcli's scopes; the paste-the-redirect-URL flow covers headless) |
| `gcli login --manual` | removed; one `gcli login` flow (browser or printed URL) covers desktop and headless |
| — | `gcli login --setup`, `gcli logout`, `gcli whoami` (new) |
| `sync push --concurrency/-c N` | drive-persistent `--workers/-w N` (default 4) |
| per-command `--id/-i <string>` | drive-persistent bool `--id` with uniform path resolution |
| `drive list` (ID column always; `--filter/-F`) | no ID column; `--with-id` opts in; `--filter/-f` |
| `drive search` (full-text default; `--sort/-s`, `--created-in/-C`, `--updated-in/-U`) | name-match default (`--content` for full text); `--sort` (no shorthand); `--created`, `--modified` |
| `drive upload` (+ `--overwrite`) | alias `up`; overwrite-by-name is the default, no `--overwrite` flag; `--keep-revision` |
| `drive download` | alias `dl`; atomic + MD5-verified; `--format`, `--revision` |
| `download --zip` | dropped — Drive has no server-side folder zip; `zip -r` locally instead |
| `drive copy` / `cp` | removed |
| — | `drive cat <path>` (new) |
| `drive delete <path>` (alias `rm`) | `drive rm <path>...` (aliases `remove`, `delete`; variadic) |
| `drive purge <path>` | `drive purge <path>...` (variadic) |
| `drive restore <path>` | `drive trash restore <name>` |
| `drive permissions list/create/delete`, `drive link create/get/remove` | `drive share` / `drive unshare` (`--with`, `--anyone`, `--role`, `--expires`, `--all`) |
| `drive sync push <local> <remote>` | `drive sync <local> <remote>` |
| `drive sync pull <remote> <local>` | `drive sync <local> <remote> --reverse/-R` (argument order flips to always `<local> <remote>`) |
| `sync --delete` | removed — mirror is the default; `--backup`/`--dry-run`/`--yes` are the valves |
| `sync --ignore/-i` (exact basename) | `--ignore/-i` glob patterns, declared once |
| — | `sync --backup`, `sync --yes/-y` (new) |
| — | file revisions via `info --revisions`, `download --revision`, `upload --keep-revision` (new) |
| `mail list [count]` | `mail list --limit/-n 20` |
| `mail search --max` | `mail search --limit/-n` |
| `mail send --attach/-A` | `--attach/-a` (also on reply/forward) |
| `mail forward` (no attachments) | `mail forward --attach/-a` + originals re-attached |
| `--shared/-S` | `--shared/-s`, upgraded to shared drives + shared-with-me |
| — | `--draft` on send/reply/forward; `mail drafts list/get/edit/send/rm` (new) |

## Output Modes & Scripting

Every command speaks three mutually exclusive tiers, set by root flags:

| Tier | Flag | Behavior |
|------|------|----------|
| Human | (default) | lipgloss icons/colors, transient progress that clears to a summary, terminal-bounded tables, interactive prompts |
| AI | `--for-ai` | `[OK] [INFO] [WARN] [ERROR] [RUNNING] [PROGRESS]` prefixes, append-only (nothing cleared), lossless markdown tables, piped stdin for every prompt, full error chains |
| Debug | `--debug` | zerolog structured logging to stderr |

**Stream split (all tiers):** data (tables, links, IDs, `cat` bytes, and the `[OK]/[INFO]/[RUNNING]/[PROGRESS]` lifecycle) goes to **stdout**; diagnostics (`[ERROR]/[WARN]` and debug logs) go to **stderr**. So `gcli ... --for-ai 2>err.log` yields clean, parseable stdout.

**Exit codes:**

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General failure |
| 2 | Usage error (bad args/flags, or wrong-type: sync file↔folder mismatch, `cat` on a folder) |
| 3 | Auth failure |
| 4 | Not found (path/ID/thread/draft) |
| 5 | Permission denied |
| 6 | Partial success (some items in a batch failed) |
| 7 | Rate-limited after retries exhausted |
| 130 | Cancelled (Ctrl+C / context cancelled) |

**The invisible rule:** every prompt has a flag or stdin equivalent, so `--for-ai` is fully non-interactive. Deletes take `--yes`; the login code and mail bodies read from piped stdin; ambiguity resolves via `--id`.

```bash
# fully headless: env-var auth, piped redirect URL (or bare code), no prompts
export GCLI_CLIENT_ID=... GCLI_CLIENT_SECRET=...
printf '%s\n' "$REDIRECT_URL" | gcli --for-ai login
gcli drive sync ~/backup /backups --yes --for-ai
gcli drive cat /reports/latest.json --for-ai | jq .
```

## Tips and Notes

- **`upload` never deletes; `sync` does.** `upload` is `cp` — it adds and overwrites by name but leaves remote-only files alone. `sync` is a destructive mirror and is the only command that removes divergences.
- **Sync deletes are recoverable and symmetric.** A push (`local → remote`) sends removed remote files to **Drive trash**; a `--reverse` pull sends removed local files to **`.trash.gcli/`** in the invocation directory. Use `--dry-run` to preview and `--backup` to snapshot the destination to `<name>.bak` first.
- **`rm` vs `purge`.** `rm` trashes (recoverable; Drive auto-purges trash after 30 days). `purge` deletes permanently and bypasses trash — it always confirms (or requires `--yes`).
- **Restore an old file version** with `download --revision <id>` then `upload` (which overwrites the head by default). List revision IDs with `info --revisions`. There is no separate restore API.
- **Duplicate remote names on download** get non-destructive ` (1)`, ` (2)` suffixes; the same applies to case-fold collisions on macOS/Windows filesystems.
- **Shortcuts** are labeled `shortcut` in listings and never silently followed: file-shortcuts resolve one hop on download; folder-shortcuts are reported but not recursed; sync skips them.
- **Google Workspace files** (Docs/Sheets/Slides) have no raw bytes — download/`cat` export them (`--format`), and sync counts and skips them rather than deleting. `download --revision` is **not supported** for them (the API can't export a specific historical version); `info --revisions` still lists their history, and `--format` exports the current version.
- **Retries are built in.** Transient API errors and rate limits are retried with backoff; only after retries are exhausted does a command exit 7.
- Use `GCLI_CONFIG_DIR` to keep separate config directories per account — that is the supported multi-account mechanism.
