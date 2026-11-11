# Consul IO CLI

Consul IO is a CLI tool used to import and export configuration files from a specified directory to/from the Consul KV store.

## Installation

You can install the latest version using the `go install` command:

```sh
go install github.com/turknet/consul-io@latest
```

## Check Version

```sh
consul-io version
```

## Usage

You can run the CLI tool using the following command:

```sh
consul-io help
```
### Available Commands

- `import [directory]` : Upload config files to Consul KV store
- `export [directory]` : Download config files from Consul KV store
- `vault-search [search-term]` : Search for a term in Vault KV store
- `version` : Print the version number of Consul IO
- `help` : Display help for consul-io

### Command Line Arguments

- `--consul-addr` : Specifies the address of the Consul server. The default value is `http://localhost:8500`.
- `--ignore` : Specifies one or more paths to ignore during the import process. This option is useful if you want to skip certain directories or files.
- `--state-file` : Path to the sync state (manifest) file for local use. Defaults to a per-directory file under the user config directory.
- `--full` : Forces a full scan and compares every file against Consul (local controlled resync). Ignores the state file.
- `--git-diff` : Only import files changed since the last applied commit (uses `git diff`). Recommended for CI/CD.
- `--changed-since` : Explicit git base ref/SHA to diff against (implies `--git-diff`).
- `--marker-key` : Consul key storing the last applied commit SHA, e.g. `_consul-io/main/last-applied` (implies `--git-diff`).
- `--prune` : Deletes Consul keys for files removed/renamed in git. **Disabled by default for safety.**
- `--max-delete` : Maximum number of deletions allowed per run (default `1`, `0` = unlimited). The run aborts before applying anything if exceeded.
- `--dry-run` : Prints the planned Put/Delete operations without applying them.
- `--token` : Optional ACL token for Consul authentication. If provided, all operations will be authenticated using this token.
- `--vault-addr` : Vault server address (e.g. http://vault:8200)
- `--auth-type` : Authentication type for Vault (ldap)
- `--username` : Username for Vault authentication
- `--password` : Password for Vault authentication
- `--path` : Optional specific path to search in Vault
- `[directory]` : The directory containing the configuration files you want to upload or the directory to which you want to export files.

### Example Usage

#### Import
```sh
consul-io --consul-addr=http://localhost:8500 --token=my-secret-token import test --ignore="test/team1/apps/project2"
```
This command uploads the configuration files in the `test` directory to the Consul KV store.

### Incremental Import (Change Detection)

`import` supports two change-detection modes:

**1. Local mode (manifest based)** — used when no git flags are given:

- Keeps a small **state manifest** (relative path -> content hash) of the last
  successful sync and only processes files that were added, modified, deleted or
  renamed since then.
- First run (no state file): full scan, all files are uploaded.
- Subsequent runs: only changed files cause Consul API calls. Unchanged files
  cause zero Consul operations (only a local directory scan).
- Rename detection is based on identical content hashes.
- Only successfully processed files are recorded in the state; failed operations
  are retried on the next run (never marked as processed).
- If the state file is missing, corrupted, or belongs to a different Consul
  address, the tool safely falls back to a full scan.
- Use `--full` for a controlled resync.

**2. CI/CD mode (git diff based)** — used with `--git-diff` / `--marker-key`:

- The base commit is read from `--changed-since`, or from a Consul **marker key**
  (`--marker-key`) that stores the last successfully applied commit SHA.
- Only files changed in `<base>..HEAD` under the import directory are processed:
  added/modified are uploaded; deleted/renamed are removed **only if `--prune`**.
- If the base commit is not present locally (e.g. shallow clone) or no marker
  exists yet, the tool falls back to a full scan **without any deletion**.
- The marker is advanced to `HEAD` **only if every operation succeeded**;
  otherwise the next run retries from the same base (no lost delta).
- Marker keys live under the reserved `_consul-io/` prefix and are skipped by
  `export`, so they never leak into the config repo.

**Safety around deletions**

- Deletions are disabled by default; they require an explicit `--prune`.
- `--max-delete` (default `1`) aborts the whole run before applying anything if
  the planned number of deletions exceeds the limit.
- `--dry-run` previews the exact Put/Delete plan.
- On the full-scan fallback, deletions are never performed.

#### Bitbucket Pipelines (main only)

Hazır bir şablon: [`examples/bitbucket-pipelines.yml`](examples/bitbucket-pipelines.yml)
(tam klon, otomatik `--dry-run` önizleme ve manuel onaylı uygulama adımı).

```sh
$EXEC_PATH/consul-io --consul-addr=https://consul-addr \
  import dev --git-diff --marker-key _consul-io/main/last-applied --prune
```

#### Vault Search
```sh
# Search in all KV paths
consul-io vault-search "search-term" \
  --vault-addr="http://vault:8200" \
  --auth-type="ldap" \
  --username="your-username" \
  --password="your-password"

# Search in a specific path
consul-io vault-search "search-term" \
  --vault-addr="http://vault:8200" \
  --auth-type="ldap" \
  --username="your-username" \
  --password="your-password" \
  --path="secret/specific/path"
```
This command searches for the specified term in Vault KV store and displays matching values with colored output:
- Path is shown in green
- Keys are shown in yellow
- Values are shown in red

#### Export
```sh
consul-io --consul-addr=http://localhost:8500 --token=my-secret-token export test
```
This command downloads the configuration files from the Consul KV store and saves them in the `test` directory, maintaining the same structure.

### Colorful Console Output

Consul IO provides colorful console output to improve readability:

- `Errors` are displayed in `red`.
- `Warnings` are displayed in `yellow`.
- `Success messages` are displayed in `green`.
- `Informational messages` are displayed in `cyan`.


## Example Directory Structure

```go
consul-io/
├── cmd/
│   └── root.go
├── test/
│   ├── team1/apps/
│   │   ├── project1/
│   │   │   └── .env.production
│   │   ├── project2/
│   │   │   └── appsettings.json.production
│   ├── team2/apps/
│   │   ├── project1/
│   │   │   └── .env.production
│   │   │   └── appsettings.json.production
├── go.mod
├── go.sum
└── main.go
```

- `cmd/` : Contains the CLI command files.

- `go.mod` : Contains Go module information.

- `go.sum` : Contains the verification information for the Go modules.

- `main.go` : The entry point of the program.

- `test/` : Example directory containing the configuration files you want to upload or the directory where you want to export files.

## Dependencies

- [github.com/hashicorp/consul/api](https://github.com/hashicorp/consul/api) : Library used to interact with the Consul API.

- [github.com/spf13/cobra](https://github.com/spf13/cobra) : Library used to create the command line interface.

- [github.com/fatih/color](https://github.com/fatih/color)  : Library used for adding color to the terminal output.
