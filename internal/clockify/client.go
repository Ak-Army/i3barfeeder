package clockify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// requestTimeout bounds every API call: without it a stuck connection blocks the
// module goroutine forever, holding the Clockify module lock with it.
const requestTimeout = 10 * time.Second

type Client struct {
	client   *http.Client
	baseUrl  string
	apiToken string
}

func NewClient(apiToken string) Client {
	return Client{
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				// A custom dialer disables HTTP/2 unless it is asked for explicitly.
				ForceAttemptHTTP2: true,
				HTTP2: &http.HTTP2Config{
					SendPingTimeout: 15 * time.Second,
					PingTimeout:     10 * time.Second,
				},
				MaxIdleConns:          4,
				IdleConnTimeout:       60 * time.Second,
				TLSHandshakeTimeout:   5 * time.Second,
				ExpectContinueTimeout: time.Second,
			},
			Timeout: requestTimeout,
		},
		baseUrl:  "https://api.clockify.me/api/v1",
		apiToken: apiToken,
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
		c.client.CloseIdleConnections()
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
