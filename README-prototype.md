# Nginx root package prototype - build + serve

PROTOTYPE ONLY. Local branch `nginx-root-prototype` in `~/Workspace/elastic/{package-spec,elastic-package,package-registry,integrations}`. Nothing here is pushed.

What's patched:

- `package-spec` - `schemas` (`default` + schema -> `requires`) and `requires.integration` in the integration manifest spec. Semantic check: `requires.integration` only on packages without policy templates or data streams. Test package `test/packages/good_root`.
- `elastic-package` - `go.mod` `replace github.com/elastic/package-spec/v3 => ../package-spec`. New profile setting `stack.epr.base_image` (default: stock EPR image).
- `package-registry` - `schemas` and `requires.integration` exposed on `/search` and `/package/...`.

## 1. Build the patched tools

```sh
# package-spec (sanity)
cd ~/Workspace/elastic/package-spec
make check

# elastic-package - rebuild every time package-spec changes
cd ~/Workspace/elastic/elastic-package
go build -o bin/elastic-package-proto .

# package-registry image used as the stack's EPR base
cd ~/Workspace/elastic/package-registry
docker build -t package-registry:nginx-root-prototype .
```

## 2. Profile

The profile `nginx-proto` is already there. To recreate it:

```sh
EP=~/Workspace/elastic/elastic-package/bin/elastic-package-proto
$EP profiles create nginx-proto
echo 'stack.epr.base_image: package-registry:nginx-root-prototype' \
  > ~/.elastic-package/profiles/nginx-proto/config.yml
```

## 3. Build the packages

```sh
EP=~/Workspace/elastic/elastic-package/bin/elastic-package-proto
cd ~/Workspace/elastic/integrations/packages/nginx
$EP build                                  # root -> nginx_group-0.1.0
(cd ecs          && $EP build)             # nginx-3.2.2
(cd otel         && $EP build)             # nginx_otel_integ-0.1.0
(cd otel_input   && $EP build)             # nginx_otel_input-0.2.2
(cd otel_content && $EP build)             # nginx_otel-0.6.1
(cd ../filelog_otel && $EP build)          # filelog_otel-0.2.0
```

Zips land in `~/Workspace/elastic/integrations/build/packages/`. Building from a nested child dir works with no workaround.

## 4. Start the stack

```sh
cd ~/Workspace/elastic/integrations/packages/nginx
script -q /dev/null ~/Workspace/elastic/elastic-package/bin/elastic-package-proto \
  stack up -v -d -p nginx-proto --version 9.6.0-SNAPSHOT
```

- Run it from inside the integrations repo. `stack up` serves whatever is in `integrations/build/packages/`.
- That dir also holds stale zips from earlier builds (e.g. `nginx-2.0.0`, `elastic_connectors-*`, `logstash-2.7.1`). They get served too. We left them on purpose (Kyle hasn't decided yet).
- `-v` is needed with Docker Compose v5.5.x. Without it you get `failed to get console: provided file is not a console`: elastic-package sends compose stdout to `io.Discard` and `compose build` wants a console. The `script -q /dev/null` wrapper only matters when there is no TTY (agents, CI). In a normal terminal, `stack up -v ...` is enough.
- The EPR container is built `FROM package-registry:nginx-root-prototype`, because of `stack.epr.base_image`. It serves the local zips and proxies everything else to https://epr.elastic.co.
- Stop with `$EP stack down -p nginx-proto`.

## 5. Endpoints + credentials

| What | URL | Auth |
|---|---|---|
| Elasticsearch | https://127.0.0.1:9200 | `elastic` / `changeme` |
| Kibana (container) | https://127.0.0.1:5601 | `elastic` / `changeme` |
| Package registry | https://127.0.0.1:8080 | none |
| Fleet Server | https://127.0.0.1:8220 | - |

- CA for all of them: `~/.elastic-package/profiles/nginx-proto/certs/ca-cert.pem`. The certs cover `localhost` and `127.0.0.1`.
- Kibana service account token (what the container Kibana uses):

  ```sh
  grep serviceAccountToken ~/.elastic-package/profiles/nginx-proto/stack/kibana.yml
  ```

- Shell env for elastic-package or curl: `eval "$($EP stack shellinit -p nginx-proto)"`.

Verify the root:

```sh
CA=~/.elastic-package/profiles/nginx-proto/certs/ca-cert.pem
curl -s --cacert $CA "https://127.0.0.1:8080/search?package=nginx_group&prerelease=true" | jq '.[0].schemas'
curl -s --cacert $CA "https://127.0.0.1:8080/package/nginx_group/0.1.0/" | jq '.schemas'
```

## 6. Install the children

Install them in this order (otel requires otel_input, filelog_otel, and otel_content):

```sh
EP=~/Workspace/elastic/elastic-package/bin/elastic-package-proto
cd ~/Workspace/elastic/integrations/packages
for d in nginx/otel_input nginx/otel_content filelog_otel nginx/ecs nginx/otel; do
  (cd $d && $EP install -p nginx-proto)
done
```

The root `nginx_group` is not installed. It only exists in the registry.

## 7. Point a Kibana dev server at this stack

The Kibana source is `~/Workspace/elastic/kibana` (`yarn start`). Put this in `config/kibana.dev.yml` and replace `<TOKEN>` with the token from step 5:

```yaml
elasticsearch.hosts: ["https://localhost:9200"]
elasticsearch.serviceAccountToken: "<TOKEN>"
elasticsearch.ssl.certificateAuthorities: ["/Users/kylepollich/.elastic-package/profiles/nginx-proto/certs/ca-cert.pem"]

xpack.fleet.registryUrl: "https://localhost:8080"
xpack.fleet.agents.fleet_server.hosts: ["https://localhost:8220"]
xpack.fleet.internal.skipUploadPackageValidation: true
xpack.fleet.experimentalFeatures:
  enableOtelIntegrations: true
```

Start it with Node trusting the stack CA, so Fleet can reach the HTTPS registry:

```sh
cd ~/Workspace/elastic/kibana
NODE_EXTRA_CA_CERTS=~/.elastic-package/profiles/nginx-proto/certs/ca-cert.pem yarn start
```

Then log in as `elastic` / `changeme`.

Notes:

- Kibana refuses the `elastic` superuser in `elasticsearch.username`. Use the service account token, not user/password.
- The dev Kibana and the container Kibana share the same ES and `.kibana*` indices. Kibana main can run saved-object migrations the 9.6.0-SNAPSHOT container doesn't know about. Before `yarn start`, stop the container Kibana: `docker stop elastic-package-stack-nginx-proto-kibana-1`. Fleet Server and the agent keep running. Bring it back with `docker start` on the same container.
- If Kibana main has moved past 9.6.0, ES may reject it. Bump `--version` on `stack up`.
- When Kibana is on the host, the registry is `https://localhost:8080`. Inside the compose network it is `https://package-registry:8080`.

## Known issues

- **ECS + OTel template collision.** `nginx_otel_integ` declares `dataset: nginx.access` / `nginx.error`. Fleet names the index templates `logs-nginx.access` / `logs-nginx.error` (pattern `*.otel-*`), and those are the same names `nginx` (ECS) uses. Whichever child is installed last owns the template and the `@package` component template. After installing `otel`, the ECS `nginx` logs templates are overwritten. Reinstalling `ecs` then fails with `illegal_argument_exception ... composable template [logs-nginx.access] ... is invalid`. The fix belongs in integrations (otel dataset names) or in Fleet (template name for `.otel` data streams). It is not fixed here.
- `stack up` without `-v` fails on Compose v5.5.x (see step 4).
- The stale zips in `integrations/build/packages` are served (see step 4).
