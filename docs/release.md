# Release

## CI

`.github/workflows/build.yml` runs on every push to `main` and on pull
requests. It runs `go test ./...`, then builds static binaries
(`CGO_ENABLED=0`) with `make build` for:

| OS     | Arch          |
|--------|---------------|
| linux  | amd64, arm64  |
| darwin | amd64, arm64  |

The binaries are attached to each run as artifacts named
`gorxiv-<os>-<arch>`.

## Cutting a release

```sh
git tag v0.1.0
git push origin v0.1.0
```

On a `v*` tag the workflow also creates a GitHub release with the four
binaries attached. The `Makefile` reads the tag (without the `v`) into
`cmd.version`, so `gorxiv --version` prints `0.1.0`.

## Nix

```sh
nix run github:btraven00/gorxiv -- --help   # run without installing
nix build                                   # ./result/bin/gorxiv
nix develop                                 # shell with go and gopls
```

The flake builds for x86_64/aarch64 Linux and macOS. Its version is the git
short revision (`-dirty` for uncommitted trees).

### Updating `vendorHash`

`flake.nix` pins a hash of the Go module dependencies. After any change to
`go.mod`/`go.sum`, `nix build` fails with a hash mismatch:

```
error: hash mismatch in fixed-output derivation '...-go-modules.drv':
         specified: sha256-...
            got:    sha256-NEWHASH
```

Copy the `got:` value into `vendorHash` in `flake.nix` and rebuild.
