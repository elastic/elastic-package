package profile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateProfile(t *testing.T) {
	err := CreateProfile(t.TempDir())
	require.NoError(t, err)

}
