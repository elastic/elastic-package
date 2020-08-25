package ingestmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/stack"
)

type Policy struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Namespace   string `json:"namespace"`
}

func (c *Client) CreatePolicy(p Policy) (*Policy, error) {
	reqBody, err := json.Marshal(p)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert policy (request) to JSON")
	}

	statusCode, respBody, err := c.post("agent_policies", bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.Wrap(err, "could not create policy")
	}

	if statusCode != 200 {
		return nil, fmt.Errorf("could not create policy; API status code = %d", statusCode)
	}

	var resp struct {
		Item Policy `json:"item"`
	}

	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, errors.Wrap(err, "could not convert policy (response) to JSON")
	}

	return &resp.Item, nil
}

func (c *Client) DeletePolicy(p Policy) error {
	reqBody := `{ "agentPolicyId": "` + p.ID + `" }`

	statusCode, _, err := c.post("agent_policies/delete", strings.NewReader(reqBody))
	if err != nil {
		return errors.Wrap(err, "could not delete policy")
	}

	if statusCode != 200 {
		return fmt.Errorf("could not delete policy; API status code = %d", statusCode)
	}

	return nil
}

func (c *Client) AddPackageToPolicy(p Policy, pkg Package) error {
	json := `
{
  "name":"apache-1",
  "description":"",
  "namespace":"ep",
  "policy_id":"5cbde330-e672-11ea-b5be-15057523bf1c",
  "enabled":true,
  "output_id":"",
  "inputs":[
    {
      "type":"logfile",
      "enabled":true,
      "streams":[
        {
          "id":"logfile-apache.access",
          "enabled":true,
          "data_stream":{
            "type":"logs",
            "dataset":"apache.access"
          },
          "vars":{
            "paths":{
              "value":[
                "/var/log/apache2/access.log*",
                "/var/log/apache2/other_vhosts_access.log*",
                "/var/log/httpd/access_log*"
              ],
              "type":"text"
            }
          }
        },
        {
          "id":"logfile-apache.error",
          "enabled":true,
          "data_stream":{
            "type":"logs",
            "dataset":"apache.error"
          },
          "vars":{
            "paths":{
              "value":[
                "/var/log/apache2/error.log*",
                "/var/log/httpd/error_log*"
              ],
              "type":"text"
            }
          }
        }
      ]
    },
    {
      "type":"apache/metrics",
      "enabled":true,
      "streams":[
        {
          "id":"apache/metrics-apache.status",
          "enabled":true,
          "data_stream":{
            "type":"metrics",
            "dataset":"apache.status"
          },
          "vars":{
            "period":{
              "value":"10s",
              "type":"text"
            },
            "server_status_path":{
              "value":"/server-status",
              "type":"text"
            }
          }
        }
      ],
      "vars":{
        "hosts":{
          "value":[
            "http://127.0.0.1"
          ],
          "type":"text"
        }
      }
    }
  ],
  "package":{
    "name":"apache",
    "title":"Apache",
    "version":"0.1.4"
  }
}`
}

func (c *Client) post(resourcePath string, reqBody io.Reader) (int, []byte, error) {
	url := c.apiBaseUrl + "/" + resourcePath
	req, err := http.NewRequest(http.MethodPost, url, reqBody)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "could not create POST request to Ingest Manager resource: %s", resourcePath)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Add("content-type", "application/json")
	req.Header.Add("kbn-xsrf", stack.DefaultVersion)

	_, statusCode, respBody, err := sendRequest(req)
	if err != nil {
		return statusCode, respBody, errors.Wrapf(err, "error sending POST request to Ingest Manager resource: %s", resourcePath)
	}

	return statusCode, respBody, nil
}

func sendRequest(req *http.Request) (*http.Response, int, []byte, error) {
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, errors.Wrap(err, "could not send request to Kibana API")
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp, resp.StatusCode, nil, errors.Wrap(err, "could not read response body")
	}

	return resp, resp.StatusCode, body, nil
}
