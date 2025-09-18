# How to Develop Using Podman Instead of Docker

Podman is a container engine that is compatible with Docker but does not require a daemon and can run containers rootless. If you want to use Podman for development instead of Docker, follow these steps:

## 1. Install Podman

Refer to the [official Podman installation guide](https://podman.io/getting-started/installation) for your operating system.

## 2. Configure Docker Compatibility

Podman provides a Docker-compatible CLI. You can setup Podman so it runs whenever the docker commands are run, [follow the migration guide](https://podman-desktop.io/docs/migrating-from-docker/managing-docker-compatibility)

## 3. Run Docker Commands with Podman

Most Docker commands work with Podman. For example:

```sh
podman build -t my-image .
podman run -it my-image
podman ps
```

## 4. Use Podman Compose

If your project uses `docker-compose`, install [podman-compose](https://github.com/containers/podman-compose):

```sh
pip install podman-compose
```

Then use:

```sh
podman-compose up
```

## 5. Troubleshooting and Compatibility

- Podman is mostly compatible with Docker CLI and images.
- Some advanced Docker features may require adjustments.
- For more details, see the [Podman documentation](https://docs.podman.io/).

## 6. Additional Resources

- [Podman Official Documentation](https://docs.podman.io/)
- [Podman vs Docker Comparison](https://podman.io/getting-started/)