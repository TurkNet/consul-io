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
- `version` : Print the version number of Consul IO
- `help` : Display help for consul-io

### Command Line Arguments

- `--consul-addr` : Specifies the address of the Consul server. The default value is `http://localhost:8500`.

- `[directory]` : The directory containing the configuration files you want to upload or the directory to which you want to export files.

### Example Usage

#### Import
```sh
consul-io --consul-addr=http://localhost:8500 import test
```
This command finds the files with the `.production` extension in the `test` directory and uploads them to the Consul KV store.



#### Export
```sh
consul-io --consul-addr=http://localhost:8500 export test
```
This command downloads the configuration files from the Consul KV store and saves them in the `test` directory, maintaining the same structure.

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