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

// IndicesGetComponentTemplateService returns a component template.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.10/getting-component-templates.html
// for more details.
type IndicesGetComponentTemplateService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	name          []string
	masterTimeout string
	flatSettings  *bool
	local         *bool
}

// NewIndicesGetComponentTemplateService creates a new IndicesGetComponentTemplateService.
func NewIndicesGetComponentTemplateService(client *Client) *IndicesGetComponentTemplateService {
	return &IndicesGetComponentTemplateService{
		client: client,
		name:   make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesGetComponentTemplateService) Pretty(pretty bool) *IndicesGetComponentTemplateService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesGetComponentTemplateService) Human(human bool) *IndicesGetComponentTemplateService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesGetComponentTemplateService) ErrorTrace(errorTrace bool) *IndicesGetComponentTemplateService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesGetComponentTemplateService) FilterPath(filterPath ...string) *IndicesGetComponentTemplateService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesGetComponentTemplateService) Header(name string, value string) *IndicesGetComponentTemplateService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesGetComponentTemplateService) Headers(headers http.Header) *IndicesGetComponentTemplateService {
	s.headers = headers
	return s
}

// Name is the name of the component template.
func (s *IndicesGetComponentTemplateService) Name(name ...string) *IndicesGetComponentTemplateService {
	s.name = append(s.name, name...)
	return s
}

// FlatSettings is returns settings in flat format (default: false).
func (s *IndicesGetComponentTemplateService) FlatSettings(flatSettings bool) *IndicesGetComponentTemplateService {
	s.flatSettings = &flatSettings
	return s
}

// Local indicates whether to return local information, i.e. do not retrieve
// the state from master node (default: false).
func (s *IndicesGetComponentTemplateService) Local(local bool) *IndicesGetComponentTemplateService {
	s.local = &local
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesGetComponentTemplateService) MasterTimeout(masterTimeout string) *IndicesGetComponentTemplateService {
	s.masterTimeout = masterTimeout
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesGetComponentTemplateService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string
	if len(s.name) > 0 {
		path, err = uritemplates.Expand("/_component_template/{name}", map[string]string{
			"name": strings.Join(s.name, ","),
		})
	} else {
		path = "/_component_template"
	}
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
	if s.flatSettings != nil {
		params.Set("flat_settings", fmt.Sprintf("%v", *s.flatSettings))
	}
	if s.local != nil {
		params.Set("local", fmt.Sprintf("%v", *s.local))
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesGetComponentTemplateService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *IndicesGetComponentTemplateService) Do(ctx context.Context) (*IndicesGetComponentTemplateResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "GET",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	var ret *IndicesGetComponentTemplateResponse
	if err := s.client.decoder.Decode(res.Body, &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// IndicesGetComponentTemplateResponse is the response of IndicesGetComponentTemplateService.Do.
type IndicesGetComponentTemplateResponse struct {
	ComponentTemplates []IndicesGetComponentTemplates `json:"component_templates"`
}

type IndicesGetComponentTemplates struct {
	Name              string                       `json:"name"`
	ComponentTemplate *IndicesGetComponentTemplate `json:"component_template"`
}

type IndicesGetComponentTemplate struct {
	Version  int                              `json:"version,omitempty"`
	Template *IndicesGetComponentTemplateData `json:"template,omitempty"`
}

type IndicesGetComponentTemplateData struct {
	Settings map[string]interface{} `json:"settings,omitempty"`
	Mappings map[string]interface{} `json:"mappings,omitempty"`
	Aliases  map[string]interface{} `json:"aliases,omitempty"`
}
