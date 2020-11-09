package export

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/kibana/dashboards"
)

func Dashboards(kibanaDashboardsClient *dashboards.Client, dashboardsIDs []string) error {
	objects, err := kibanaDashboardsClient.Export(dashboardsIDs)
	if err != nil {
		return errors.Wrap(err, "exporting dashboards using Kibana client failed")
	}

	for _, object := range objects {
		id, _ := object.GetValue("id")
		aType, _ := object.GetValue("type")
		fmt.Println(id, aType)
	}
	return nil
}
