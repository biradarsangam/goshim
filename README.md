# goshim



## Install

```sh
go get github.com/biradarsangam/goshim/go-shim
```

The default build uses the Docker-compatible containerd registry backend.
Build with the `podman` tag to use the Podman containers/image backend.
`NewPushSession` is available with the default backend.

## Use

```go
package main

import (
	"context"
	"fmt"

	goshim "github.com/biradarsangam/goshim/go-shim"
)

func main() {
	ctx := context.Background()

	changed, err := goshim.Push(ctx, "./oci-layout", "registry.example.com/team/model:v1")
	if err != nil {
		panic(err)
	}
	fmt.Println("uploaded new content:", changed)
}
```

The package also exposes `Pull`, `Inspect`, `Transfer`, `ResolveDigest`,
`PushArtifact`, `Referrers`, `Sign`, `Verify`, `Login`, `Logout`,
`SetRegistryMirrors`, and (with the default backend) `NewPushSession` for
streamed uploads. Network operations accept a `context.Context` so callers can
cancel or time-bound them.

## ModelPack

The ModelPack-aware APIs use the official
[ModelPack specification](https://github.com/modelpack/model-spec). They
validate the OCI manifest and ModelPack configuration in an existing layout;
they do not package model files or build the layout.

```go
package main

import (
	"context"
	"fmt"

	goshim "github.com/biradarsangam/goshim/go-shim"
)

func main() {
	ctx := context.Background()
	ref := "registry.example.com/team/model:v1"

	artifact, err := goshim.InspectModelLayout("./oci-layout", ref)
	if err != nil {
		panic(err)
	}
	fmt.Println("model:", artifact.Model.Descriptor.Name)

	changed, err := goshim.PushModel(ctx, "./oci-layout", ref)
	if err != nil {
		panic(err)
	}
	fmt.Println("uploaded new content:", changed)
}
```

`PullModel` downloads an artifact and validates it as ModelPack; if validation
fails, the downloaded layout remains available. The generic `Push` and `Pull`
continue to support non-ModelPack OCI artifacts.

## Run a Linux workload with libkrun

`goshim-run` runs a command in a libkrun microVM using a prepared Linux root
filesystem for the host architecture. Libkrun is the only execution backend.
The `go-shim` package handles OCI registry operations separately.

Build with cgo, a C compiler, `pkg-config`, and
[libkrun 1.x](https://github.com/libkrun/libkrun/releases) (1.19.4 or later),
with the matching libkrunfw available to the dynamic loader. The runner supports
Linux amd64/arm64 with access to `/dev/kvm` and macOS on Apple Silicon.

```sh
CGO_ENABLED=1 go build -tags libkrun -o goshim-run ./cmd/goshim-run
```

On macOS, sign the binary after building it. If libkrunfw is installed outside
the loader's search path, point `DYLD_FALLBACK_LIBRARY_PATH` to its library
directory, such as `/opt/homebrew/lib`.

```sh
codesign --force --sign - --entitlements cmd/goshim-run/hypervisor.entitlements ./goshim-run
```

```sh
./goshim-run --rootfs ./rootfs --cpus 2 --memory-mib 512 -- /bin/sh -c 'uname -a'
```

Use a dedicated rootfs directory. The guest can write to it through virtio-fs.
The guest has no external network interface and starts with a minimal
environment instead of inheriting host variables. The runner passes through
the guest's exit status. Libkrun 1.x passes arguments via the kernel command
line, so use short UTF-8 arguments without double quotes or control whitespace
and an absolute guest executable path without spaces.

## Test

```sh
go test ./...
```

For the Podman backend:

```sh
go test -tags podman,containers_image_openpgp,exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper ./...
```

The library is distributed under the Apache-2.0 license; see [LICENSE](./LICENSE).
