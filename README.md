# Consul Uploader CLI

Consul Uploader is a CLI tool used to upload configuration files from a specified directory to the Consul KV store.

## Installation

You can download the latest release from the [Releases](https://github.com/turknet/consul-uploader/releases) page.

### Using the Binary

Download the binary for your platform from the releases page and add it to your PATH.

#### Example for Linux

```sh
wget https://github.com/turknet/consul-uploader/releases/download/v1.0.1/consul-uploader-linux-amd64
chmod +x consul-uploader-linux-amd64
sudo mv consul-uploader-linux-amd64 /usr/local/bin/consul-uploader
```

#### Example for macOS

```sh
wget https://github.com/turknet/consul-uploader/releases/download/v1.0.0/consul-uploader-darwin-amd64
chmod +x consul-uploader-darwin-amd64
sudo mv consul-uploader-darwin-amd64 /usr/local/bin/consul-uploader
```

#### Example for macOS
Download the binary from the releases page and add it to your PATH.


## Usage

You can run the CLI tool using the following command:

```sh
consul-uploader --consul-addr=http://localhost:8500 /path/to/config/directory
```


### Installing Dependencies for Local Development

To install the dependencies for the project, run the following commands:

```sh
go get github.com/hashicorp/consul/api
go get github.com/spf13/cobra
go mod tidy
```

## Usage for Local Development

You can run the CLI tool using the following command:

```sh
go run main.go --consul-addr=http://localhost:8500 /path/to/config/directory
```

### Command Line Arguments

* `--consul-addr`  : Specifies the address of the Consul server. The default value is `http://localhost:8500`.

* `/path/to/config/directory` : The directory containing the configuration files you want to upload.

## Example Usage

```sh
go run main.go --consul-addr=http://localhost:8500 test
```

This command finds the files with the `.production` extension in the `test` directory and uploads them to the Consul KV store.

## Example Directory Structure

```go
consul-uploader/
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

* `cmd/:` Contains the CLI command files.

* `test/:` Example directory containing the configuration files you want to upload.

* `go.mod:` Contains Go module information.

* `go.sum:` Contains the verification information for the Go modules.

* `main.go:` The entry point of the program.

## Bağımlılıklar

* [github.com/hashicorp/consul/api](https://github.com/hashicorp/consul/api) : Library used to interact with the Consul API.

* [github.com/spf13/cobra](https://github.com/spf13/cobra) : Library used to create the command line interface.