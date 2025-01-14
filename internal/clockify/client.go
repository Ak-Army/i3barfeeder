package clockify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Ak-Army/xlog"
)

type Client struct {
	client    *http.Client
	transport *http.Transport
	baseUrl   string
	apiToken  string
}

// M2MzZGYyZGYtM2E0My00Y2I5LTg4NmItNzEyYmU3ZmVmMWIy
func NewClient(apiToken string) Client {
	transport := &http.Transport{}
	baseUrl := "https://api.clockify.me/api/v1"

	return Client{
		client:    &http.Client{Transport: transport},
		transport: transport,
		baseUrl:   baseUrl,
		apiToken:  apiToken,
	}
}
func (c Client) request(method string, endpoint string, param interface{}) (response []byte, err error) {
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

	xlog.Infof("Requesting %s %s", method, c.baseUrl+endpoint)
	res, err := c.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	contentType := res.Header.Get("content-type")
	if !(res.StatusCode >= 200 && res.StatusCode < 300) {
		err = fmt.Errorf("response wrong status code: %d", res.StatusCode)
		response, _ = ioutil.ReadAll(res.Body)
	} else if strings.Contains(contentType, "application/json") {
		response, err = ioutil.ReadAll(res.Body)
	} else {
		err = errors.New("response wrong content type")
	}
	return
}
