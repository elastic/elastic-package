package dashboards

import (
	"encoding/json"
	"github.com/elastic/elastic-package/internal/logger"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/stack"
)

type exportedType struct {
	Objects []common.MapStr `json:"objects"`
}

type Client struct {
	host     string
	username string
	password string
}

func NewClient() (*Client, error) {
	host := os.Getenv(stack.KibanaHostEnv)
	if host == "" {
		return nil, stack.UndefinedEnvError(stack.ElasticsearchHostEnv)
	}

	username := os.Getenv(stack.ElasticsearchUsernameEnv)
	password := os.Getenv(stack.ElasticsearchPasswordEnv)

	return &Client{
		host:     host,
		username: username,
		password: password,
	}, nil
}

func (c *Client) Export(dashboardIDs []string) ([]common.MapStr, error) {
	logger.Debug("Export dashboards using the Kibana Export API")

	var query strings.Builder
	query.WriteByte('?')
	for _, dashboardID := range dashboardIDs {
		query.WriteString("dashboard=")
		query.WriteString(dashboardID)
		query.WriteByte('&')
	}

	request, err := http.NewRequest(http.MethodGet, c.host+"/api/kibana/dashboards/export"+query.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "building HTTP request failed")
	}
	request.SetBasicAuth(c.username, c.password)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "sending HTTP request failed")
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading response body failed")
	}

	var exported exportedType
	err = json.Unmarshal(body, &exported)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling response failed (body: \n%s)", string(body))
	}

	var multiErr multierror.Error
	for _, obj := range exported.Objects {
		errMsg, err := obj.GetValue("error.message")
		if errMsg != nil {
			multiErr = append(multiErr, errors.New(errMsg.(string)))
			continue
		}
		if err != nil && err != common.ErrKeyNotFound {
			multiErr = append(multiErr, err)
		}
	}

	if len(multiErr) > 0 {
		return nil, errors.Wrap(multiErr, "at least Kibana object returned an error")
	}
	return exported.Objects, nil
}
