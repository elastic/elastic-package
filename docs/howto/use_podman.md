# How to Develop Using Podman Instead of Docker

`elastic-package` Docker-based commands can work with Podman instead of Docker.
The tool shells out to the `docker` and `docker compose` CLI commands, so the
simplest path is to keep the Docker CLI installed and point it at the Podman
socket via the `DOCKER_HOST` environment variable.

## Podman Desktop (macOS)

If you are using Podman Desktop on macOS, Docker compatibility is enabled by
default. See the [Podman Desktop migration guide](https://podman-desktop.io/docs/migrating-from-docker/managing-docker-compatibility)
for details.

## Native Podman (Linux)

On Linux distributions that provide Podman as a package (e.g. Ubuntu, Fedora),
no GUI is needed. The following steps assume you already have the Docker CLI and
Docker Compose v2 installed.

### 1. Install Podman

Use your distribution's package manager. For example, on Ubuntu:

```console
sudo apt install podman
```

### 2. Enable the Podman socket

Podman is daemonless by default. Enable the user-level socket so that the Docker
CLI and Compose can communicate with it:

```console
systemctl --user enable --now podman.socket
```

This creates a socket at `$XDG_RUNTIME_DIR/podman/podman.sock` (typically
`/run/user/$UID/podman/podman.sock`).

On some distributions (including Ubuntu), `systemd-tmpfiles-clean` may remove the
socket file during its daily cleanup. If the socket disappears while the system is
running, add an exclusion rule:

```console
echo 'x /run/user/*/podman/*' | sudo tee /etc/tmpfiles.d/podman-user.conf
```

Then restart the socket:

```console
systemctl --user restart podman.socket
```

### 3. Set `DOCKER_HOST`

Export the variable in your shell profile (e.g. `~/.bashrc`, `~/.zshrc`):

```bash
export DOCKER_HOST=unix:///run/user/$(id -u)/podman/podman.sock
```

The Docker CLI and Docker Compose both honour this variable and will talk to the
Podman socket. No alias from `docker` to `podman` is needed — in fact, aliasing
would be counterproductive because `elastic-package` calls `docker compose` as a
subcommand of the Docker CLI, and shell aliases do not propagate to child
processes.

The [Podman Desktop](https://podman-desktop.io/) flatpak can be installed
alongside this set-up to provide a GUI for managing containers. It connects to
the same podman socket and does not interfere with the CLI configuration.

### 4. Verify

```console
docker info          # should show Podman as the runtime
docker compose version   # should show Compose v2+
```

## Links

- [Podman Desktop](https://podman-desktop.io/)
- [Podman Desktop Docs](https://podman-desktop.io/docs/intro)
- [Using the `DOCKER_HOST` environment variable](https://podman-desktop.io/docs/migrating-from-docker/using-the-docker_host-environment-variable)
