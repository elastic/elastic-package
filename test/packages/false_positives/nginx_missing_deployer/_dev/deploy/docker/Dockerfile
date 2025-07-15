ARG SERVICE_VERSION=${SERVICE_VERSION:-1.22.1}
FROM nginx:${SERVICE_VERSION}
RUN apt-get update && apt-get install -y curl
HEALTHCHECK --interval=1s --retries=90 CMD curl -f http://localhost/server-status
COPY ./nginx.conf /etc/nginx/
