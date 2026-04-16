# Fiber — Go SDK

Plugin-authoring helpers and service SDKs for the [Pulp](https://github.com/BananaLabs-OSS/Pulp) application runtime.

The Go SDK lives on this branch. Other language SDKs are on their own branches:
- `go` — this branch
- `java` — Java SDK

The `main` branch is pure documentation.

## Packages

- **`pulp/`** — plugin-authoring helpers. Drop-in glue for writing Pulp plugins in Go: the required WASM exports (`pulp_alloc`, `pulp_init`, `pulp_step`, `pulp_shutdown`), typed request/response envelopes, and thin wrappers over host imports (HTTP, fs, sqlite).
- **`pulp/gin/`** — Gin-compatible router that runs inside a Pulp plugin. Lets existing Gin handler code run unchanged — only the bootstrap (`gin.Default() → pulpgin.New()`) changes.

## Install

```sh
go get github.com/BananaLabs-OSS/Fiber@go
```

Then import the subpackage you need:

```go
import "github.com/BananaLabs-OSS/Fiber/pulp"
import pulpgin "github.com/BananaLabs-OSS/Fiber/pulp/gin"
```

Build any plugin with:

```sh
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
```

## Hello world

```go
package main

import pulpgin "github.com/BananaLabs-OSS/Fiber/pulp/gin"

func main() {}

func init() {
	r := pulpgin.New()
	r.GET("/hello/:name", func(c *pulpgin.Context) {
		c.JSON(200, pulpgin.H{"hello": c.Param("name")})
	})
	_ = r.Run()
}
```

Manifest (`pulp.plugin.toml`):

```toml
name = "hello"
version = "0.1.0"
wasm = "hello.wasm"

capabilities = ["transport.http.inbound"]
```
