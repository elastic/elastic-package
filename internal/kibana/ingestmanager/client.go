package ingestmanager

type Client struct {
	apiBaseUrl string

	username string
	password string
}

func NewClient(baseUrl, username, password string) (*Client, error) {
	return &Client{
		baseUrl + "/api/ingest_manager",
		username,
		password,
	}, nil
}
