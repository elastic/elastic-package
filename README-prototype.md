# Nginx root package prototype - build + serve

PROTOTYPE ONLY. Branch `nginx-root-prototype` in `~/Workspace/elastic/{package-spec,elastic-package,package-registry,integrations,kibana}`. Draft PRs, for viewing diffs only: elastic/package-spec#1272, elastic/elastic-package#4053, elastic/package-registry#2174, elastic/integrations#21939, elastic/kibana#296761.

What's patched:

- `package-spec` - `schemas` (`default` + schema -> `requires`) and `requires.integration` in the integration manifest spec. Semantic check: `requires.integration` only on packages without policy templates or data streams. Test package `test/packages/good_root`.
- `elastic-package` - no code changes. It's built against the local package-spec through an untracked `go.work`, so `go.mod` doesn't change.
- `package-registry` - `schemas` and `requires.integration` exposed on `/search` and `/package/...`. Run from source; no custom Docker image.
- `kibana` - Fleet UI for roots (one tile, OTel/ECS toggle).

## 1. Build the patched tools

```sh
# package-spec (sanity)
cd ~/Workspace/elastic/package-spec
make check

# elastic-package against the local spec. go.work is untracked (listed in .git/info/exclude).
# Rebuild every time package-spec changes - the spec is embedded in the binary.
cd ~/Workspace/elastic/elastic-package
go work init . ../package-spec   # once
go build -o bin/elastic-package-proto .

# package-registry
cd ~/Workspace/elastic/package-registry
go build -o package-registry .
```

Check that the binary uses the local spec: from `integrations/packages/nginx`, a stock elastic-package fails with `Additional property schemas is not allowed`, and `elastic-package-proto` doesn't.

## 2. Build the packages

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

## 3. Start the stack

Uses the stock profile and stock EPR image. Dev Kibana doesn't use the stack's EPR (see step 4).

```sh
EP=~/Workspace/elastic/elastic-package/bin/elastic-package-proto
$EP profiles create nginx-proto   # once
$EP stack up -v -d -p nginx-proto --version 9.6.0-SNAPSHOT
docker stop elastic-package-stack-nginx-proto-kibana-1   # dev Kibana replaces it
```

- `-v` is needed with Docker Compose v5.5.x. Without it you get `failed to get console: provided file is not a console`. Without a TTY (agents, CI), wrap the command: `script -q /dev/null $EP stack up ...`.
- Stop the container Kibana before starting dev Kibana. They share ES, and Kibana main can run saved-object migrations that the 9.6.0-SNAPSHOT container doesn't know about.
- Stop the stack with `$EP stack down -p nginx-proto`.

## 4. Run package-registry locally

Create `config.nginx-proto.yml` in the package-registry repo. It's untracked and listed in `.git/info/exclude`:

```yaml
package_paths:
  - /Users/kylepollich/Workspace/elastic/integrations/build/packages
cache_time.index: 10s
cache_time.search: 10s
cache_time.categories: 10s
cache_time.catch_all: 10s
```

```sh
cd ~/Workspace/elastic/package-registry
./package-registry -address localhost:8081 -config config.nginx-proto.yml \
  -feature-proxy-mode=true -proxy-to=https://epr.elastic.co/ \
  -disable-package-validation -require-package-signatures=false
```

- Port 8081, because the stack EPR has 8080.
- Proxy mode serves anything not built locally (`fleet_server`, `system`, ...) from epr.elastic.co, same as the stack EPR.
- Locally built zips aren't signed, so `-require-package-signatures=false` is required.
- It indexes at startup. Restart it after rebuilding packages.

Verify the root:

```sh
curl -s "localhost:8081/search?package=nginx_group&prerelease=true" | jq '.[0].schemas'
curl -s "localhost:8081/package/nginx_group/0.1.0/" | jq '.schemas'
```

## 5. Dev Kibana

`config/kibana.dev.yml` sets `server.basePath: /kyle`. The Kibana encryption key must match the stack Kibana's. Pass it in a config file, because on the CLI the all-digit key is parsed as a number.

```sh
cd ~/Workspace/elastic/kibana
CA=~/.elastic-package/profiles/nginx-proto/certs/ca-cert.pem
S=<scratch dir>
grep '^xpack.encryptedSavedObjects.encryptionKey' ~/.elastic-package/profiles/nginx-proto/stack/kibana.yml > $S/kbn-proto.yml
NODE_EXTRA_CA_CERTS=$CA node scripts/kibana --dev -c config/kibana.yml -c $S/kbn-proto.yml \
  --no-base-path --server.rewriteBasePath=true --server.port=5602 \
  --xpack.security.authc.providers.basic.basic.order=0 \
  --elasticsearch.hosts=https://localhost:9200 \
  --elasticsearch.username=kibana_system --elasticsearch.password=changeme \
  --elasticsearch.ssl.certificateAuthorities=$CA \
  --xpack.fleet.registryUrl=http://localhost:8081 \
  --xpack.fleet.internal.skipUploadPackageValidation=true
```

Open http://localhost:5602/kyle/app/integrations/browse?q=nginx and log in as `elastic` / `changeme`.

Gotchas:

- Wrong encryption key -> every policy deploy spends ~14s failing to decrypt `fleet-message-signing-keys`.
- `--dev` conflicts with the service account token. Use `kibana_system` / `changeme` instead (that password is set on the stack ES).
- Without `rewriteBasePath=true`, URLs return 400. Without `basic.order=0`, `--dev` injects a SAML provider that has no realm, and you get an auth error page.

## 6. Endpoints

| What | URL | Auth |
|---|---|---|
| Elasticsearch | https://localhost:9200 | `elastic` / `changeme` |
| Package registry (local) | http://localhost:8081 | none |
| Kibana (dev) | http://localhost:5602/kyle | `elastic` / `changeme` |
| Fleet Server | https://localhost:8220 | - |

CA for the stack: `~/.elastic-package/profiles/nginx-proto/certs/ca-cert.pem`.

## Known issues

- `stack up` without `-v` fails on Compose v5.5.x (see step 3).
- `elastic-package lint` on the root fails with `item [demo] is not allowed in folder [_dev]`. `build` is fine.
