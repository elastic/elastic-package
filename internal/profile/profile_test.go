package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateProfile(t *testing.T) {
	options := Options{
		PackagePath: t.TempDir(),
	}
	err := CreateProfile(options)
	require.NoError(t, err)

}
