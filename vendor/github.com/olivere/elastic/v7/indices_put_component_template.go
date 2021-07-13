// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// IndicesPutComponentTemplateService creates or updates component templates.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.10/indices-component-template.html
// for more details on this API.
type IndicesPutComponentTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name          string
	create        *bool
	cause         string
	masterTimeout string

	bodyJson   interface{}
	bodyString string
}

// NewIndicesPutComponentTemplateService creates a new IndicesPutComponentTemplateService.
func NewIndicesPutComponentTemplateService(client *Client) *IndicesPutComponentTemplateService {
	return &IndicesPutComponentTemplateService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesPutComponentTemplateService) Pretty(pretty bool) *IndicesPutComponentTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesPutComponentTemplateService) Human(human bool) *IndicesPutComponentTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesPutComponentTemplateService) ErrorTrace(errorTrace bool) *IndicesPutComponentTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesPutComponentTemplateService) FilterPath(filterPath ...string) *IndicesPutComponentTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesPutComponentTemplateService) Header(name string, value string) *IndicesPutComponentTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesPutComponentTemplateService) Headers(headers http.Header) *IndicesPutComponentTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the component template.
func (s *IndicesPutComponentTemplateService) Name(name string) *IndicesPutComponentTemplateService {
	s.name = name
	return s
}

// Create indicates whether the component template should only be added if
// new or can also replace an existing one.
func (s *IndicesPutComponentTemplateService) Create(create bool) *IndicesPutComponentTemplateService {
	s.create = &create
	return s
}

// Cause is the user-defined reason for creating/updating the the component template.
func (s *IndicesPutComponentTemplateService) Cause(cause string) *IndicesPutComponentTemplateService {
	s.cause = cause
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesPutComponentTemplateService) MasterTimeout(masterTimeout string) *IndicesPutComponentTemplateService {
	s.masterTimeout = masterTimeout
	return s
}

// BodyJson is the component template definition as a JSON serializable
// type, e.g. map[string]interface{}.
func (s *IndicesPutComponentTemplateService) BodyJson(body interface{}) *IndicesPutComponentTemplateService {
	s.bodyJson = body
	return s
}

// BodyString is the component template definition as a raw string.
func (s *IndicesPutComponentTemplateService) BodyString(body string) *IndicesPutComponentTemplateService {
	s.bodyString = body
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesPutComponentTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_component_template/{name}", map[string]string{
		"name": s.name,
	})
	if err != nil {
		return "", url.Values{}, err
	}

	// Add query string parameters
	params := url.Values{}
	if v := s.pretty; v != nil {
		params.Set("pretty", fmt.Sprint(*v))
	}
	if v := s.human; v != nil {
		params.Set("human", fmt.Sprint(*v))
	}
	if v := s.errorTrace; v != nil {
		params.Set("error_trace", fmt.Sprint(*v))
	}
	if len(s.filterPath) > 0 {
		params.Set("filter_path", strings.Join(s.filterPath, ","))
	}
	if s.create != nil {
		params.Set("create", fmt.Sprint(*s.create))
	}
	if s.cause != "" {
		params.Set("cause", s.cause)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesPutComponentTemplateService) Validate() error {
	var invalid []string
	if s.name == "" {
		invalid = append(invalid, "Name")
	}
	if s.bodyString == "" && s.bodyJson == nil {
		invalid = append(invalid, "BodyJson")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IndicesPutComponentTemplateService) Do(ctx context.Context) (*IndicesPutComponentTemplateResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Setup HTTP request body
	var body interface{}
	if s.bodyJson != nil {
		body = s.bodyJson
	} else {
		body = s.bodyString
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "PUT",
		Path:    path,
		Params:  params,
		Body:    body,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(IndicesPutComponentTemplateResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesPutComponentTemplateResponse is the response of IndicesPutComponentTemplateService.Do.
type IndicesPutComponentTemplateResponse struct {
	Acknowledged       bool   `json:"acknowledged"`
	ShardsAcknowledged bool   `json:"shards_acknowledged"`
	Index              string `json:"index,omitempty"`
}
