package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrimStringSlice(t *testing.T) {
	strs := []string{"foo bar ", "  bar baz", "\tbaz qux\t\t", "qux foo"}
	expected := []string{"foo bar", "bar baz", "baz qux", "qux foo"}

	TrimStringSlice(strs)
	require.Equal(t, expected, strs)
}
