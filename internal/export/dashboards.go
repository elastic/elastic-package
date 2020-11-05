package export

import "github.com/elastic/elastic-package/internal/kibana/dashboards"

func Dashboards(kibanaDashboardsClient *dashboards.Client, dashboardsIDs []string) error {
	return nil
}
