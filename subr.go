// subr.go

/*
Package ssllabs contains SSLLabs-related functions.
*/
package ssllabs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"strings"
	"time"

	"net/http"

	"github.com/pkg/errors"
)

func myRedirect(req *http.Request, via []*http.Request) error {
	return nil
}

// AddQueryParameters adds query parameters to the URL.
func AddQueryParameters(baseURL string, queryParams map[string]string) string {
	params := url.Values{}
	if len(queryParams) == 0 {
		return baseURL
	}
	for key, value := range queryParams {
		params.Add(key, value)
	}
	return fmt.Sprintf("%s?%s", baseURL, params.Encode())
}

// prepareRequest insert all pre-defined stuff
func (c *Client) prepareRequest(method, what string, opts map[string]string) (req *http.Request) {
	var endPoint string

	// This allow for overriding baseurl for tests
	if c.baseurl != "" {
		endPoint = fmt.Sprintf("%s/%s", c.baseurl, what)
	} else {
		endPoint = fmt.Sprintf("%s/%s", baseURL, what)
	}

	c.verbose("Options:\n%v", opts)
	baseURL := AddQueryParameters(endPoint, opts)
	c.debug("baseURL: %s", baseURL)

	req, _ = http.NewRequest(method, baseURL, nil)

	c.debug("req=%#v", req)

	// We need these when we POST
	if method == "POST" {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
	}

	return
}

func (c *Client) callAPI(word, sbody string, opts map[string]string) ([]byte, error) {
	var body []byte

	retry := 0

	c.debug("callAPI")
	req := c.prepareRequest(word, "GET", opts)
	if req == nil {
		return []byte{}, errors.New("req is nil")
	}

	c.debug("clt=%#v", c.client)
	c.debug("opts=%v", opts)

	resp, err := c.client.Do(req)
	if err != nil {
		c.debug("err=%#v", err)
		return []byte{}, errors.Wrap(err, "1st call")
	}
	defer resp.Body.Close()

	c.debug("resp=%#v", resp)

	for {
		if retry == c.retries {
			return nil, errors.New("retries")
		}

		c.debug("read body")
		body, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return []byte{}, errors.Wrapf(err, "body read, retry=%d", retry)
		}

		c.debug("body=%v", string(body))

		if resp.StatusCode == http.StatusOK {

			c.debug("status OK")

			// We wait for FINISHED state
			if !strings.Contains(string(body), "FINISHED") {
				time.Sleep(2 * time.Second)
				retry++
				resp, err = c.client.Do(req)
				if err != nil {
					return body, errors.Wrapf(err, "pending, retry=%d", retry)
				}
				c.debug("resp was %v", resp)
			} else {
				return body, nil
			}
		} else if resp.StatusCode == http.StatusFound {
			str := resp.Header["Location"][0]

			c.debug("Got 302 to %s", str)

			req := c.prepareRequest(word, "GET", opts)
			if err != nil {
				return []byte{}, errors.Wrap(err, "redirect")
			}

			resp, err = c.client.Do(req)
			retry++
			if err != nil {
				return []byte{}, errors.Wrap(err, "client.Do failed")
			}
			c.debug("resp was %v", resp)
		} else {
			return body, errors.Wrapf(err, "status: %v body: %q", resp.Status, body)
		}
	}
	return body, err
}

// Display for one report
func (rep *LabsReport) String() {
	host := rep.Host
	if len(rep.Endpoints) != 0 {
		grade := rep.Endpoints[0].Grade
		//details := rep.Endpoints[0].Details
		log.Printf("Looking at %s — grade %s", host, grade)
	}
}

// ParseResults unmarshals the json payload
func ParseResults(content []byte) (r []LabsReport, err error) {
	var data []LabsReport

	err = json.Unmarshal(content, &data)
	return data, errors.Wrap(err, "unmarshal")
}
