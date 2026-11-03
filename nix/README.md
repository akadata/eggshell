# Nix helpers for eggshell userland

Builds `rootfs/amd64.tar` from the latest nixpkgs (nixos-unstable), from the
project root:

```sh
nix-build nix
cp result/*.tar rootfs/
make
```

## Options

  * `--arg withURoot false` leaves out u-root
  * `--arg nixpkgs '<nixpkgs>'` uses your channel instead of nixos-unstable
  * `--arg sshAuthorizedKeys '[ "ssh-ed25519 AAAA..." ]'` sets the root keys
    accepted by the SSH server, otherwise no authentication is required
  * `--arg withGo true` adds the Go toolchain (about 130 MB)
  * `--arg withTests true` adds the test programs (`../userland/tests`) and
    scripts under `/usr/libexec/tests`

Contents:

|                                         | path in the archive                       |
|-----------------------------------------|-------------------------------------------|
| BusyBox (applets in `/bin`, `/sbin`)    | `/bin/busybox`, `/usr/bin/env`            |
| bash                                    | `/bin/bash`                               |
| redis (no TLS, no systemd)              | `/usr/bin/redis-{server,cli,benchmark}`   |
| musl dynamic loader                     | `/lib/ld-musl-<arch>.so.1`,`/lib/libc.so` |
| OpenSSH `sftp-server` (for `scp`, `sftp`) | `/usr/libexec/sftp-server`              |
| u-root                                  | `/bbin/bb`, `/bbin/<command>`             |
| Go toolchain (`withGo`)                 | `/usr/local/go`, `/usr/bin/{go,gofmt}`    |
| `../userland/skel`                      | configuration override                    |

`/bbin` is not in the kernel's default `PATH`: run `/bbin/gosh`, then `export
PATH=/bbin:$PATH` to prefer the u-root commands.
