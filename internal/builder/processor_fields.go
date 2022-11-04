package builder

import _ "embed"

//go:embed _static/field_agent_processor.yml
var agentFieldsProcessor []byte

//go:embed _static/field_host_processor.yml
var hostFieldsProcessor []byte

//go:embed _static/field_geo_processor.yml
var geoFieldsProcessor []byte

//go:embed _static/field_cloud_processor.yml
var cloudFieldsProcessor []byte

var fieldProcessorsFiles = [][]byte{
	agentFieldsProcessor,
	hostFieldsProcessor,
	geoFieldsProcessor,
	cloudFieldsProcessor,
}

func fieldsProcessorsData() ([]byte, error) {
	var allFiles []byte

	for _, data := range fieldProcessorsFiles {
		allFiles = append(allFiles, data...)
	}
	return allFiles, nil
}
