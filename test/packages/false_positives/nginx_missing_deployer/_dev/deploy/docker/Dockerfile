ARG SERVICE_VERSION=${SERVICE_VERSION:-1.19.5}
FROM nginx:${SERVICE_VERSION}
# move to archive
RUN sed -i 's/deb.debian/archive.debian/' /etc/apt/sources.list && \
    sed -i 's/security.debian/archive.debian/' /etc/apt/sources.list && \
    sed -i "/buster-updates/d" /etc/apt/sources.list
RUN apt-get update && apt-get install -y curl
HEALTHCHECK --interval=1s --retries=90 CMD curl -f http://localhost/server-status
COPY ./nginx.conf /etc/nginx/
