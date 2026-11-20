# goshim



## Install

```sh
go get github.com/biradarsangam/goshim
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

	"github.com/biradarsangam/goshim"
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

	"github.com/biradarsangam/goshim"
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

## Test

```sh
go test ./...
```

For the Podman backend:

```sh
go test -tags podman,containers_image_openpgp,exclude_graphdriver_btrfs,exclude_graphdriver_devicemapper ./...
```

The library is distributed under the Apache-2.0 license; see [LICENSE](./LICENSE).
