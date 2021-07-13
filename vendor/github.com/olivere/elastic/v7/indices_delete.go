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

// IndicesDeleteService allows to delete existing indices.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-delete-index.html
// for details.
type IndicesDeleteService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	timeout           string
	masterTimeout     string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
}

// NewIndicesDeleteService creates and initializes a new IndicesDeleteService.
func NewIndicesDeleteService(client *Client) *IndicesDeleteService {
	return &IndicesDeleteService{
		client: client,
		index:  make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesDeleteService) Pretty(pretty bool) *IndicesDeleteService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesDeleteService) Human(human bool) *IndicesDeleteService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesDeleteService) ErrorTrace(errorTrace bool) *IndicesDeleteService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesDeleteService) FilterPath(filterPath ...string) *IndicesDeleteService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesDeleteService) Header(name string, value string) *IndicesDeleteService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesDeleteService) Headers(headers http.Header) *IndicesDeleteService {
	s.headers = headers
	return s
}

// Index adds the list of indices to delete.
// Use `_all` or `*` string to delete all indices.
func (s *IndicesDeleteService) Index(index []string) *IndicesDeleteService {
	s.index = index
	return s
}

// Timeout is an explicit operation timeout.
func (s *IndicesDeleteService) Timeout(timeout string) *IndicesDeleteService {
	s.timeout = timeout
	return s
}

// MasterTimeout specifies the timeout for connection to master.
func (s *IndicesDeleteService) MasterTimeout(masterTimeout string) *IndicesDeleteService {
	s.masterTimeout = masterTimeout
	return s
}

// IgnoreUnavailable indicates whether to ignore unavailable indexes (default: false).
func (s *IndicesDeleteService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesDeleteService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard expression
// resolves to no concrete indices (default: false).
func (s *IndicesDeleteService) AllowNoIndices(allowNoIndices bool) *IndicesDeleteService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards indicates whether wildcard expressions should get
// expanded to open or closed indices (default: open).
func (s *IndicesDeleteService) ExpandWildcards(expandWildcards string) *IndicesDeleteService {
	s.expandWildcards = expandWildcards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesDeleteService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/{index}", map[string]string{
		"index": strings.Join(s.index, ","),
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
	if s.timeout != "" {
		params.Set("timeout", s.timeout)
	}
	if s.masterTimeout != "" {
		params.Set("master_timeout", s.masterTimeout)
	}
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesDeleteService) Validate() error {
	var invalid []string
	if len(s.index) == 0 {
		invalid = append(invalid, "Index")
	}
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *IndicesDeleteService) Do(ctx context.Context) (*IndicesDeleteResponse, error) {
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
		Method:  "DELETE",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(IndicesDeleteResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a delete index request.

// IndicesDeleteResponse is the response of IndicesDeleteService.Do.
type IndicesDeleteResponse struct {
	Acknowledged bool `json:"acknowledged"`
}
