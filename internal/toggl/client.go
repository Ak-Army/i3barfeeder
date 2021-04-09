package toggl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
)

type Client struct {
	client    *http.Client
	transport *http.Transport
	baseUrl   string
	apiToken  string
}

func NewClient(apiToken string) Client {
	transport := &http.Transport{}
	baseUrl := "https://toggl.com/api/v8"

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
	basic := base64.StdEncoding.EncodeToString([]byte(c.apiToken + ":api_token"))
	req.Header.Set("Authorization", "Basic "+basic)

	res, err := c.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	contentType := res.Header.Get("content-type")
	if !(res.StatusCode >= 200 && res.StatusCode < 300) {
		err = errors.New("response wrong status code")
	} else if strings.Contains(contentType, "application/json") {
		response, err = ioutil.ReadAll(res.Body)
	} else {
		err = errors.New("response wrong content type")
	}
	return
}
