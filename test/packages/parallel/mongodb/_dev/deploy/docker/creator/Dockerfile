ARG SERVICE_VERSION=${SERVICE_VERSION:-7.0}
FROM mongo:${SERVICE_VERSION}
RUN apt-get update && apt-get install -y netcat-openbsd
HEALTHCHECK --interval=1s --retries=90 CMD nc -z localhost 27017