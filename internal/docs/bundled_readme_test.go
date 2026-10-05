// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchesBundledRegions(t *testing.T) {
	cases := []struct {
		title    string
		existing string
		segments []string
		want     bool
	}{
		{"no bundled regions, equal", "a", []string{"a"}, true},
		{"no bundled regions, different", "b", []string{"a"}, false},
		{"bundled region content is ignored", "# T\nwhatever\n## F\nanything\n", []string{"# T\n", "\n## F\n", "\n"}, true},
		{"empty bundled regions", "# T\n## F\n", []string{"# T\n", "## F\n"}, true},
		{"adjacent bundled regions", "# T\nxyz\n", []string{"# T\n", "", "\n"}, true},
		{"bundled regions at start and end", "xx\nmiddle\nyy", []string{"", "\nmiddle\n", ""}, true},
		{"prefix differs", "# Other\nfoo\n", []string{"# T\n", "\n"}, false},
		{"middle segment missing", "# T\nfoo\nbar\n", []string{"# T\n", "\n## F\n", "\n"}, false},
		{"suffix differs", "# T\nfoo\n## F\nbar\nextra", []string{"# T\n", "\n## F\n", "\n"}, false},
		{"segments out of order", "# T\n## F\nfoo\n## G\n", []string{"# T\n", "\n## G\n", "\n## F\n"}, false},
	}
	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			assert.Equal(t, c.want, matchesBundledRegions(c.existing, c.segments))
		})
	}
}

func TestCollapseBundledRegions(t *testing.T) {
	want := "# T\n\nProse.\n\n" + bundledPlaceholder + "\n\n## F\n\n" + bundledPlaceholder + "\n\nEnd.\n"

	t.Run("edit away from the regions keeps it", func(t *testing.T) {
		existing := "# T\n\nOld prose.\n\nline 1\nline 2\n\n## F\n\n| a | b |\n| c | d |\n\nEnd.\n"
		expected := "# T\n\nOld prose.\n\n" + bundledPlaceholder + "\n\n## F\n\n" + bundledPlaceholder + "\n\nEnd.\n"
		assert.Equal(t, expected, collapseBundledRegions(existing, want))
	})

	t.Run("edit next to a region is not hidden", func(t *testing.T) {
		existing := "# T\n\nProse.\n\nline 1\nline 2\n\n## F\n\n| a | b |\n\nChanged end.\n"
		collapsed := collapseBundledRegions(existing, want)
		assert.Contains(t, collapsed, "Changed end.")
		assert.NotContains(t, collapsed, "line 1")
	})

	t.Run("without regions nothing changes", func(t *testing.T) {
		assert.Equal(t, "a\nb\n", collapseBundledRegions("a\nb\n", "a\nc\n"))
	})
}

func TestReadmeSnapshot(t *testing.T) {
	setup := func(t *testing.T) string {
		t.Helper()
		packageRoot := t.TempDir()
		createComposableManifestFile(t, packageRoot)
		createReadmeTemplateFile(t, packageRoot, "# README\n")
		return packageRoot
	}

	t.Run("non-composable package", func(t *testing.T) {
		packageRoot := t.TempDir()
		createManifestFile(t, packageRoot)
		createReadmeTemplateFile(t, packageRoot, "# README\n")
		writeCommittedReadme(t, packageRoot, "committed")

		snapshot, err := SnapshotComposableReadmes(packageRoot)
		require.NoError(t, err)
		assert.Nil(t, snapshot)

		writeCommittedReadme(t, packageRoot, "regenerated")
		assert.NoError(t, snapshot.VerifyUnchanged())
	})

	t.Run("readme not changed by the build", func(t *testing.T) {
		packageRoot := setup(t)
		writeCommittedReadme(t, packageRoot, "committed\n")

		snapshot, err := SnapshotComposableReadmes(packageRoot)
		require.NoError(t, err)
		require.NotNil(t, snapshot)

		writeCommittedReadme(t, packageRoot, "committed\n")
		assert.NoError(t, snapshot.VerifyUnchanged())
	})

	t.Run("readme regenerated with other content", func(t *testing.T) {
		packageRoot := setup(t)
		writeCommittedReadme(t, packageRoot, "outdated\n")

		snapshot, err := SnapshotComposableReadmes(packageRoot)
		require.NoError(t, err)

		writeCommittedReadme(t, packageRoot, "bundled\n")
		err = snapshot.VerifyUnchanged()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "README.md")
		assert.Contains(t, err.Error(), "-outdated")
		assert.Contains(t, err.Error(), "+bundled")
	})

	t.Run("readme didn't exist", func(t *testing.T) {
		packageRoot := setup(t)

		snapshot, err := SnapshotComposableReadmes(packageRoot)
		require.NoError(t, err)

		writeCommittedReadme(t, packageRoot, "bundled\n")
		err = snapshot.VerifyUnchanged()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "+bundled")
	})

	t.Run("readme not generated", func(t *testing.T) {
		packageRoot := setup(t)

		snapshot, err := SnapshotComposableReadmes(packageRoot)
		require.NoError(t, err)
		assert.NoError(t, snapshot.VerifyUnchanged())
	})

	t.Run("static readme without template is ignored", func(t *testing.T) {
		packageRoot := t.TempDir()
		createComposableManifestFile(t, packageRoot)
		writeCommittedReadme(t, packageRoot, "static\n")

		snapshot, err := SnapshotComposableReadmes(packageRoot)
		require.NoError(t, err)
		require.NotNil(t, snapshot)
		require.NoError(t, os.WriteFile(filepath.Join(packageRoot, "docs", "README.md"), []byte("other\n"), 0o644))
		assert.NoError(t, snapshot.VerifyUnchanged())
	})
}
