package docs

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/formatter"

	"github.com/pkg/errors"
)

const sampleEventFile = "sample_event.json"

func renderSampleEvent(packageRoot, dataStreamName string) (string, error) {
	eventPath := filepath.Join(packageRoot, "data_stream", dataStreamName, sampleEventFile)

	body, err := ioutil.ReadFile(eventPath)
	if err != nil {
		return "", errors.Wrapf(err, "reading sample event file failed (path: %s)", eventPath)
	}

	formatted, _, err := formatter.JsonFormatter(body)
	if err != nil {
		return "", errors.Wrapf(err, "formatting sample event file failed (path: %s)", eventPath)
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("An example event for `%s` looks as following:\n\n",
		stripDataStreamFolderSuffix(dataStreamName)))
	builder.WriteString("```$json\n")
	builder.Write(formatted)
	builder.WriteString("\n```")
	return builder.String(), nil
}

func stripDataStreamFolderSuffix(dataStreamName string) string {
	dataStreamName = strings.ReplaceAll(dataStreamName, "_metrics", "")
	dataStreamName = strings.ReplaceAll(dataStreamName, "_logs", "")
	return dataStreamName
}
