package clockify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// requestTimeout bounds every API call: without it a stuck connection blocks the
// module goroutine forever, holding the Clockify module lock with it.
const requestTimeout = 10 * time.Second

type Client struct {
	client    *http.Client
	transport *http.Transport
	baseUrl   string
	apiToken  string
}

func NewClient(apiToken string) Client {
	transport := &http.Transport{}
	baseUrl := "https://api.clockify.me/api/v1"

	return Client{
		client:    &http.Client{Transport: transport, Timeout: requestTimeout},
		transport: transport,
		baseUrl:   baseUrl,
		apiToken:  apiToken,
	}
}
func (c *Client) request(method string, endpoint string, param any) (response []byte, err error) {
	var bodyText []byte
	if param != nil {
		bodyText, err = json.Marshal(param)
		if err != nil {
			return
		}
	}

	req, err := http.NewRequest(method, c.baseUrl+endpoint, bytes.NewReader(bodyText))
	if err != nil {
		return
	}
	req.Header.Set("X-Api-Key", c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	//xlog.Debugf("Requesting %s %s", method, c.baseUrl+endpoint)
	res, err := c.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	contentType := res.Header.Get("content-type")
	if !(res.StatusCode >= 200 && res.StatusCode < 300) {
		err = fmt.Errorf("response wrong status code: %d", res.StatusCode)
		response, _ = io.ReadAll(res.Body)
	} else if strings.Contains(contentType, "application/json") {
		response, err = io.ReadAll(res.Body)
	} else {
		err = errors.New("response wrong content type")
	}
	return
}
