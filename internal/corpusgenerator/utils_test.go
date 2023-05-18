package corpusgenerator

import (
	"bytes"
	"github.com/elastic/elastic-integration-corpus-generator-tool/pkg/genlib"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

type mockClient struct {
}

func (c mockClient) GetGoTextTemplate(packageName, dataStreamName string) ([]byte, error) {
	return []byte("7 bytes"), nil
}
func (c mockClient) GetConf(packageName, dataStreamName string) (genlib.Config, error) {
	return genlib.Config{}, nil
}
func (c mockClient) GetFields(packageName, dataStreamName string) (genlib.Fields, error) {
	return genlib.Fields{}, nil
}

func TestGeneratorEmitTotEvents(t *testing.T) {
	generator, err := NewGenerator(mockClient{}, "packageName", "dataSetName", 7)
	assert.NoError(t, err)

	state := genlib.NewGenState()

	totEvents := 0
	buf := bytes.NewBufferString("")
	for {
		err := generator.Emit(state, buf)
		if err == io.EOF {
			break
		}

		totEvents += 1
	}

	assert.Equal(t, 1, totEvents, "expected 1 totEvents, got %d", totEvents)
}
