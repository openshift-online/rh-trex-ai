# Prerequisites

TRex requires the following tools to be pre-installed:

## crc

`crc` stands for CodeReady Containers and is a tool to run containers. It manages a local OpenShift 4.x cluster, or an OKD cluster VM optimized for testing and development purposes.

- **Purpose**: It is used for running OpenShift clusters locally.
- **Installation**: Visit the [crc documentation](https://crc.dev/crc/).

## Go

`Go` is an open-source programming language that makes it easy to build simple, reliable, and efficient software.

- **Purpose**: In our project, Go is required for building and running the `trex` binary required by TRex.
- **Installation**: Install Go from the [official Go website](https://golang.org/dl/).

## jq

`jq` is a lightweight and flexible command-line JSON processor. It is used for JSON parsing and manipulation.

- **Purpose**: It allows parsing of JSON outputs from commands. Additional information and documentation can be found on [DevDocs for jq](https://devdocs.io/jq/).
- **Installation**: Follow the instructions on the [jq official website](https://jqlang.github.io/jq/).

## ocm

`ocm` stands for OpenShift Cluster Manager and is used for managing OpenShift clusters, including creation, deletion, and configuration.

- **Purpose**: It is a CLI tool used for the managing Openshift clusters.
- **Installation**: Refer to the [OCM documentation](https://github.com/openshift-online/ocm-cli).


## buf

`buf` is a modern protocol buffer toolchain that handles linting, breaking change detection, and code generation for `.proto` files.

- **Purpose**: Required to generate Go stubs from `.proto` files (`make proto`). The generated code is gitignored, so this must be run before building.
- **Installation**: See [buf.build/docs/installation](https://buf.build/docs/installation), or `go install github.com/bufbuild/buf/cmd/buf@latest`

## protoc-gen-go and protoc-gen-go-grpc

Code generation plugins for `buf` that produce Go message types and gRPC service stubs from `.proto` files.

- **Purpose**: Used by `buf generate` (via `make proto`) to create the Go code in `pkg/api/grpc/`.
- **Installation**:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

## grpcurl (optional)

`grpcurl` is a command-line tool for interacting with gRPC servers, similar to `curl` for REST APIs.

- **Purpose**: Useful for manual testing of the gRPC API (port 9000). Uses server reflection to discover services.
- **Installation**: `go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest`

## Docker or Podman

A container runtime is required for integration tests (testcontainers) and for running PostgreSQL locally.

- **Purpose**: `make db/setup` runs PostgreSQL in a container. Integration tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go/) to spin up ephemeral containers.
- **Installation**: Install [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/docs/installation).

### Podman Workarounds

testcontainers officially supports Docker only. With Podman you may hit this error:

```
Failed to start PostgreSQL testcontainer: create container: container create: Error response from daemon:
  unable to find network with name or ID bridge: network not found: creating reaper failed
```

This is caused by the [ryuk](https://github.com/testcontainers/moby-ryuk) cleanup container. Disable it:

```bash
export TESTCONTAINERS_RYUK_DISABLED=true
```

Or add to `~/.testcontainers.properties`:

```
ryuk.disabled=true
```

If you need ryuk with Podman, see [testcontainers-go#2781](https://github.com/testcontainers/testcontainers-go/issues/2781#issuecomment-2619626043) for the socket-forwarding workaround (requires elevated permissions).

Make sure all these prerequisites are installed before running TRex.