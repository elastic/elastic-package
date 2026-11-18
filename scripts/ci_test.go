// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Run the real CI scripts with stack operations and package builds replaced by
// local commands. This exercises selection, version propagation and ZIP cleanup
// without requiring Docker or changing the developer's elastic-package profile.
func scriptFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("CI scripts require bash")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is unavailable")
	}
	root := t.TempDir()
	for _, name := range []string{"stack_parameters.sh", "stack_helpers.sh", "test-check-packages.sh", "test-build-install-zip.sh", "test-build-install-zip-file.sh"} {
		contents, err := os.ReadFile(name)
		require.NoError(t, err)
		writeScriptFile(t, root, "scripts/"+name, string(contents))
	}
	for _, name := range []string{"alert_rule_templates", "alert_rule_templates_v2"} {
		category := "other"
		if strings.HasSuffix(name, "_v2") {
			category = "parallel"
		}
		contents, err := os.ReadFile(filepath.Join("../test/packages", category, name, "manifest.yml"))
		require.NoError(t, err)
		writeScriptFile(t, root, "test/packages/parallel/"+name+"/manifest.yml", string(contents))
	}
	writeScriptFile(t, root, "test/packages/parallel/alert_rule_templates_v2.stack_version", "9.6.0-SNAPSHOT\n")
	writeScriptFile(t, root, "internal/install/stack_version.go", "const (\n DefaultStackVersion = \"9.5.5\"\n)\n")
	writeScriptFile(t, root, "scripts/gpg-pass.txt", "test\n")
	writeScriptFile(t, root, "bin/go", "#!/bin/bash\nexit 0\n")
	writeScriptFile(t, root, "bin/curl", `#!/bin/bash
case "${@: -1}" in
  */api/fleet/epm/packages/alert_rule_templates/0.0.1|*/api/fleet/epm/packages/alert_rule_templates_v2/0.0.1)
    echo '"status":"installed"';;
  *) exit 22;;
esac
`)
	writeScriptFile(t, root, "bin/yq", `#!/bin/bash
if [[ "$*" == *'.name'* ]]; then basename "$(dirname "${@: -1}")"; else echo 0.0.1; fi
`)
	writeScriptFile(t, root, "bin/elastic-package", `#!/bin/bash
echo "$*" >> calls.log
command="$1"; shift
if [[ "$command" == stack && "$1" == shellinit ]]; then
  echo 'export ELASTIC_PACKAGE_ELASTICSEARCH_USERNAME=test ELASTIC_PACKAGE_ELASTICSEARCH_PASSWORD=test ELASTIC_PACKAGE_CA_CERT=test ELASTIC_PACKAGE_KIBANA_HOST=https://test'
elif [[ "$command" == stack && "$1" == up ]]; then
  version=9.5.5
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == --version ]]; then shift; version="$1"; fi
    shift
  done
  echo "$version" > selected-stack
elif [[ "$command" == build ]]; then
  while [[ "$1" != -C ]]; do shift; done
  name=$(basename "$2")
  mkdir -p "build/packages/${name}-0.0.1"
  touch "build/packages/${name}-0.0.1.zip"
elif [[ "$command" == test && "$*" == *alert_rule_templates_v2* ]]; then
  [[ "$(cat selected-stack)" == 9.6.0-SNAPSHOT ]] || exit 42
fi
`)
	return root
}

func writeScriptFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o755))
}

func runScript(t *testing.T, root, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"scripts/" + name}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PACKAGE_TEST_TYPE=parallel", "PACKAGE_UNDER_TEST=*", "SERVERLESS=false")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	contents, err := os.ReadFile(filepath.Join(root, "calls.log"))
	require.NoError(t, err)
	return string(contents)
}

func TestZipInstallStackSelection(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		args         []string
		wantV2       bool
	}{
		{name: "working copy default", script: "test-build-install-zip.sh"},
		{name: "working copy snapshot", script: "test-build-install-zip.sh", args: []string{"9.7.0-test-SNAPSHOT"}, wantV2: true},
		{name: "ZIP default", script: "test-build-install-zip-file.sh"},
		{name: "ZIP shellinit", script: "test-build-install-zip-file.sh", args: []string{"-s"}},
		{name: "ZIP snapshot", script: "test-build-install-zip-file.sh", args: []string{"-s", "-v", "9.7.0-test-SNAPSHOT"}, wantV2: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scriptFixture(t)
			writeScriptFile(t, root, "build/packages/alert_rule_templates_v2-0.0.1.zip", "stale")
			writeScriptFile(t, root, "build/packages/unrelated-0.0.1.zip", "stale")
			calls := runScript(t, root, tc.script, tc.args...)
			var installs string
			for _, line := range strings.Split(calls, "\n") {
				if strings.HasPrefix(line, "install ") {
					installs += line + "\n"
				}
			}
			assert.Contains(t, installs, "alert_rule_templates")
			assert.NotContains(t, installs, "unrelated")
			assert.Equal(t, tc.wantV2, strings.Contains(installs, "alert_rule_templates_v2"), "%s", installs)
			if tc.wantV2 {
				assert.Contains(t, calls, "stack up -d -v --version 9.7.0-test-SNAPSHOT")
			}
		})
	}
}

func TestParallelBatchHonorsStackPins(t *testing.T) {
	root := scriptFixture(t)
	calls := runScript(t, root, "test-check-packages.sh")
	assert.Contains(t, calls, "stack up -d -v --version 9.6.0-SNAPSHOT")
	assert.Contains(t, calls, "test -C test/packages/parallel/alert_rule_templates/")
	assert.Contains(t, calls, "test -C test/packages/parallel/alert_rule_templates_v2/")
}

func TestSnapshotZipInstallWiring(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Buildkite pipeline generation requires bash")
	}
	cmd := exec.Command("bash", ".buildkite/pipeline.trigger.integration.tests.sh")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "UBUNTU_X86_64_AGENT_IMAGE=test", "MACOS_ARM_AGENT_IMAGE=test", "LINUX_GOLANG_AGENT_IMAGE=test")
	pipeline, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", pipeline)
	assert.Contains(t, string(pipeline), "integration_tests.sh -t test-build-install-zip-9x-snapshot")

	makefile, err := os.ReadFile("../Makefile")
	require.NoError(t, err)
	assert.Regexp(t, `test-build-install-zip-9x-snapshot:\n\t\./scripts/test-build-install-zip.sh 9\.[^\s]+-SNAPSHOT`, string(makefile))
	config, err := os.ReadFile("../.github/workflows/updatecli/updatecli.d/bump-latest-9x-snapshot-version.yml")
	require.NoError(t, err)
	var manifest struct {
		Targets map[string]struct {
			Spec struct {
				MatchPattern   string `yaml:"matchpattern"`
				ReplacePattern string `yaml:"replacepattern"`
			} `yaml:"spec"`
		} `yaml:"targets"`
	}
	require.NoError(t, yaml.Unmarshal(config, &manifest))
	updated := string(makefile)
	for _, target := range manifest.Targets {
		pattern, err := regexp.Compile(target.Spec.MatchPattern)
		require.NoError(t, err)
		replacement := strings.ReplaceAll(target.Spec.ReplacePattern, `{{ source "latestSnapshot" }}`, "9.99.0-test")
		updated = pattern.ReplaceAllString(updated, replacement)
	}
	assert.Contains(t, updated, "./scripts/test-stack-command.sh 9.99.0-test-SNAPSHOT")
	assert.Contains(t, updated, "./scripts/test-build-install-zip.sh 9.99.0-test-SNAPSHOT")
}

func TestSnapshotZipInstallTools(t *testing.T) {
	root := scriptFixture(t)
	runner, err := os.ReadFile("../.buildkite/scripts/integration_tests.sh")
	require.NoError(t, err)
	writeScriptFile(t, root, ".buildkite/scripts/integration_tests.sh", string(runner))
	writeScriptFile(t, root, ".buildkite/scripts/install_deps.sh", `
with_go() { echo go >> tools.log; }
with_docker() { echo docker >> tools.log; }
with_docker_compose_plugin() { echo compose >> tools.log; }
with_yq() { echo yq >> tools.log; }
`)
	writeScriptFile(t, root, ".buildkite/scripts/tooling.sh", `
repo_name() { echo elastic-package; }
buildkite_pr_branch_build_id() { echo test; }
add_bin_path() { :; }
`)
	writeScriptFile(t, root, "bin/make", "#!/bin/bash\nexit 0\n")
	cmd := exec.Command("bash", ".buildkite/scripts/integration_tests.sh", "-t", "test-build-install-zip-9x-snapshot")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"BUILDKITE_REPO=test", "UPLOAD_SAFE_LOGS=0", "SERVERLESS=false")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	tools, err := os.ReadFile(filepath.Join(root, "tools.log"))
	require.NoError(t, err)
	assert.Equal(t, "go\ndocker\ncompose\nyq\n", string(tools))
}
