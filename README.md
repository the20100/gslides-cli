# gslides — Google Slides CLI

A command-line tool for the [Google Slides API v1](https://developers.google.com/workspace/slides/api/reference/rest). Create and modify Google Slides presentations programmatically.

## Install

```bash
git clone https://github.com/the20100/g-slides-cli
cd g-slides-cli
go build -o gslides .
mv gslides /usr/local/bin/
```

## Authentication

The CLI supports two auth methods:

**Option A — Service Account** (recommended for automation):
1. Create a service account in [Google Cloud Console](https://console.cloud.google.com/iam-admin/serviceaccounts)
2. Enable the Google Slides API for your project
3. Download the JSON key file

```bash
gslides auth setup --service-account /path/to/sa.json
# or: export GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa.json
```

**Option B — OAuth2** (for interactive / personal use):
1. Create OAuth2 credentials (Desktop app) at [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Enable the Google Slides API for your project
3. Add `http://localhost:8080` as an authorized redirect URI

```bash
gslides auth setup --credentials /path/to/credentials.json
# or pass --client-id and --client-secret directly

# On a remote server (VPS) where no browser is available:
gslides auth setup --credentials /path/to/credentials.json --no-browser
```

With `--no-browser`: the CLI prints the OAuth URL. Open it in a local browser, authorize, then copy the full redirect URL from the address bar and paste it into the terminal (the page will fail to load — that's expected).

Credentials are stored in:
- macOS: `~/Library/Application Support/g-slides/config.json`
- Linux: `~/.config/g-slides/config.json`
- Windows: `%AppData%\g-slides\config.json`

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Force JSON output |
| `--pretty` | Force pretty-printed JSON output |

Output is **auto-detected**: JSON when stdout is piped, tables in a terminal.

## Commands

### `gslides auth`

```bash
gslides auth setup --service-account sa.json           # configure service account
gslides auth setup --credentials credentials.json       # configure OAuth2
gslides auth setup --credentials c.json --no-browser   # OAuth2 on remote/VPS
gslides auth status                                     # show auth status
gslides auth logout                                     # remove saved credentials
```

### `gslides presentation`

```bash
# Create a new blank presentation
gslides presentation create "Q4 Review"

# Get presentation details (ID, title, slide count, URL)
gslides presentation get <presentation-id>

# List all slides with their object IDs
gslides presentation slides <presentation-id>

# Apply a batch of updates from a JSON file
gslides presentation batch-update <id> --file requests.json

# Apply updates from stdin
echo '[{"createSlide": {"insertionIndex": 1}}]' | \
  gslides presentation batch-update <id> --stdin
```

### `gslides slide`

```bash
# Get details of a specific slide and its elements
gslides slide get <presentation-id> <slide-object-id>

# Get thumbnail URL for a slide
gslides slide thumbnail <presentation-id> <slide-object-id>
gslides slide thumbnail <presentation-id> <slide-id> --mime JPEG

# Add a new blank slide (appends to end by default)
gslides slide add <presentation-id>
gslides slide add <presentation-id> --index 0          # prepend
gslides slide add <presentation-id> --layout TITLE     # with layout

# Delete a slide
gslides slide delete <presentation-id> <slide-object-id>

# Duplicate a slide
gslides slide duplicate <presentation-id> <slide-object-id>

# Replace text throughout the entire presentation
gslides slide replace-text <id> --old "2023" --new "2024"
gslides slide replace-text <id> --old "Draft" --new "" --match-case
```

### `gslides info`

```bash
gslides info   # show binary path, config path, and auth status
```

### `gslides update`

```bash
gslides update   # pull latest from GitHub and rebuild
```

## Batch Update

The `presentation batch-update` command accepts a JSON array of [Request objects](https://developers.google.com/workspace/slides/api/reference/rest/v1/presentations/batchUpdate#Request). This is the most powerful command — it lets you do anything the API supports.

Example `requests.json`:
```json
[
  {
    "createSlide": {
      "insertionIndex": 1,
      "slideLayoutReference": {
        "predefinedLayout": "TITLE_AND_BODY"
      }
    }
  }
]
```

Available slide layouts: `BLANK`, `CAPTION_ONLY`, `TITLE`, `TITLE_AND_BODY`, `TITLE_AND_TWO_COLUMNS`, `TITLE_ONLY`, `SECTION_HEADER`, `SECTION_TITLE_AND_DESCRIPTION`, `ONE_COLUMN_TEXT`, `MAIN_POINT`, `BIG_NUMBER`.

## Tips

- **Finding IDs**: `gslides presentation slides <id>` lists all slide object IDs
- **JSON output**: use `--json` or pipe to `jq` for scripting
- **Update**: run `gslides update` to pull the latest version from GitHub
- **Env var override**: set `GOOGLE_APPLICATION_CREDENTIALS` to bypass stored config
