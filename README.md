<div align="center">
  <img src=".github/assets/logo.png" alt="gdrive Logo" width="200">
  <h1>gdrive</h1>

  <a href="https://github.com/tanq16/gdrive/actions/workflows/release.yaml"><img alt="Build Workflow" src="https://github.com/tanq16/gdrive/actions/workflows/release.yaml/badge.svg"></a>&nbsp;<a href="https://github.com/tanq16/gdrive/releases"><img alt="GitHub Release" src="https://img.shields.io/github/v/release/tanq16/gdrive"></a><br><br>
  <a href="#capabilities">Capabilities</a> &bull; <a href="#installation">Installation</a> &bull; <a href="#usage">Usage</a> &bull; <a href="#tips-and-notes">Tips & Notes</a>
</div>

---

CLI tool for Google Drive file operations. Browse, upload, download, sync, search, and manage permissions from the terminal.

## Capabilities

| Category | Commands | Description |
|----------|----------|-------------|
| Auth | `login` | OAuth2 authentication with Google Drive |
| Browse | `list`, `info` | List folder contents, view file metadata |
| Transfer | `upload`, `download` | Upload/download files and folders with progress |
| Manage | `mkdir`, `move`, `copy`, `delete` | File and folder management |
| Search | `search` | Server-side search with filters |
| Sync | `sync push`, `sync pull` | Bidirectional directory sync with concurrency |
| Index | `index`, `index search` | Build offline index and search with regex |
| Permissions | `permissions list/create/delete` | Manage file sharing permissions |

## Installation

### Binary

Download from [releases](https://github.com/tanq16/gdrive/releases):

```bash
# Linux/macOS
curl -sL https://github.com/tanq16/gdrive/releases/latest/download/gdrive-$(uname -s)-$(uname -m) -o gdrive
chmod +x gdrive
sudo mv gdrive /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/tanq16/gdrive
cd gdrive
make build
```

## Usage

### Setup

1. Create OAuth credentials in [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Download the client secret JSON to `~/.config/gdrive/credentials.json`
3. Run `gdrive login` to authenticate

### Commands

```bash
# Browse
gdrive list /Documents
gdrive info /Documents/report.pdf

# Transfer
gdrive upload ./local-file.pdf /Documents/
gdrive download /Documents/report.pdf ./

# Manage
gdrive mkdir /Documents/NewFolder
gdrive move /Documents/file.pdf /Archive/
gdrive copy /Documents/file.pdf /Backup/ --name file-copy.pdf
gdrive delete /Documents/old-file.pdf

# Search
gdrive search "quarterly report" --type file --limit 20

# Sync
gdrive sync push ./local-dir /Documents/remote-dir --concurrency 8
gdrive sync pull /Documents/remote-dir ./local-dir

# Index
gdrive index /Documents
gdrive index search "\.pdf$"

# Permissions
gdrive permissions list /Documents/shared-file.pdf
gdrive permissions create /Documents/file.pdf --type user --role reader --email user@example.com
```

All commands accept `--id` flag to use Drive file IDs directly instead of paths.

## Tips and Notes

- Use `--debug` flag for structured log output with full error details
- Path resolution requires one API call per segment; use `--id` for faster scripted access
- Google Workspace files (Docs, Sheets, Slides) are automatically exported on download
- Sync skips Google Workspace native files (no checksum available)
