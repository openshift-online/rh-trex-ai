package apiclient

type Client struct {
	config *Config

	Authorization Authorization
}

type Config struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	SelfToken    string
	TokenURL     string
	Debug        bool
}

func NewClient(config Config) (*Client, error) {
	client := &Client{
		config: &config,
	}
	client.Authorization = &authorizationMock{client: client}
	return client, nil
}

func NewClientMock(config Config) (*Client, error) {
	client := &Client{
		config: &config,
	}
	client.Authorization = &authorizationMock{client: client}
	return client, nil
}

func (c *Client) Close() {
}

type service struct {
	client *Client
}
