package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
	"mosn.io/api"
	"mosn.io/mosn/pkg/variable"
	"mosn.io/pkg/buffer"

	"mosn.io/mosn/pkg/log"
	mosnhttp "mosn.io/mosn/pkg/protocol/http"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/test/lib/utils"
)

type HttpServerConfig struct {
	Addr    string                    `json:"address"`
	Configs map[string]*ResonseConfig `json:"response_configs"`
}

func NewHttpServerConfig(config interface{}) (*HttpServerConfig, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	cfg := &HttpServerConfig{}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ResponseConfig decides what response to send.
// If a request matches the Condition, send a common repsonse.
// If not, send an error response
type ResonseConfig struct {
	Condition     *Condition       `json:"condition"`
	CommonBuilder *ResponseBuilder `json:"common_builder"`
	ErrorBuilder  *ResponseBuilder `json:"error_buidler"`
}

func (cfg *ResonseConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if cfg.Condition.Match(r) {
		cfg.CommonBuilder.Build(w)
	} else {
		cfg.ErrorBuilder.Build(w)
	}
}

type Condition struct {
	// If a request contains the expected header, matched this condition.
	// A request must have the configured header key and value, and can have others headers
	// which will be ingored.
	ExpectedHeader map[string][]string `json:"expected_header"`
	// If a request contains the unexpected header, matched failed.
	UnexpectedHeaderKey []string `json"unexpected_headerkey"`
	// If a request's method is in the expected method, matched this condition.
	ExpectedMethod []string `json:"expected_method"`
	// TODO: Add more condition
}

func (c *Condition) Match(r *http.Request) bool {
	// empty condition always matched
	if c == nil {
		return true
	}
	// verify method if configured
	if len(c.ExpectedMethod) > 0 {
		if !utils.StringIn(r.Method, c.ExpectedMethod) {
			return false
		}
	}
	// verify header
	header := r.Header
	if len(header) < len(c.ExpectedHeader) {
		return false
	}
	for key, value := range c.ExpectedHeader {
		// needs to verify the string slice sequence
		key = strings.Title(key)
		if !reflect.DeepEqual(header[key], value) {
			return false
		}
	}
	for _, key := range c.UnexpectedHeaderKey {
		if _, ok := header[key]; ok {
			return false
		}
	}
	return true
}

type ResponseBuilder struct {
	// The response status code
	StatusCode int `json:"status_code"`
	// The response header
	Header map[string][]string `json:"header"`
	// The repsonse body content
	Body string `json:"body"`
}

func (b *ResponseBuilder) Build(w http.ResponseWriter) (int, error) {
	// empty builder always try to write success
	if b == nil {
		return w.Write([]byte("mosn mock http server response"))
	}
	for k, vs := range b.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	// WriteHeader should be called after Header.Set/Add
	w.WriteHeader(b.StatusCode)
	return w.Write([]byte(b.Body))
}

type HttpClientConfig struct {
	TargetAddr string         `json:"target_address"`
	MaxConn    uint32         `json:"max_connection"`
	Request    *RequestConfig `json:"request_config"`
	Verify     *VerifyConfig  `json:"verify_config"`
}

func NewHttpClientConfig(config interface{}) (*HttpClientConfig, error) {
	b, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	cfg := &HttpClientConfig{}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil

}

// RequestConfig decides what request to send
type RequestConfig struct {
	Method  string              `json:"method"`
	Header  map[string][]string `json:"header"`
	Body    json.RawMessage     `json:"body"`
	Timeout time.Duration       `json:"timeout"` // request timeout
}

func (c *RequestConfig) BuildRequest(ctx context.Context) (api.HeaderMap, buffer.IoBuffer) {
	if c == nil {
		return buildRequest(nil, "GET", nil, nil)
	}
	if c.Method == "" {
		c.Method = "GET"
	}
	method := c.Method
	return buildRequest(ctx, method, c.Header, c.Body)
}

func buildRequest(ctx context.Context, method string, header map[string][]string, body []byte) (api.HeaderMap, buffer.IoBuffer) {
	fh := &fasthttp.RequestHeader{}
	fh.SetMethod(method)
	// to simulate pkg/stream/http/stream.go injectInternalHeaders
	variable.SetString(ctx, types.VarMethod, method)
	h := mosnhttp.RequestHeader{
		RequestHeader: fh,
	}
	for k, vs := range header {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	if len(body) > 0 {
		buf := buffer.NewIoBufferBytes(body)
		return h, buf
	}
	return h, nil

}

// VerifyConfig describes what response want
type VerifyConfig struct {
	ExpectedStatusCode int
	// if ExepctedHeader is nil, means do not care about header
	// if ExepctedHeader is exists, means resposne header should contain all the ExpectedHeader
	// If response header contain keys not in ExpectedHeader, we ignore it.
	// TODO: support regex
	ExpectedHeader map[string][]string
	// if ExpectedBody is nil, means do not care about body
	// TODO: support regex
	ExpectedBody []byte
	// if MaxExpectedRT is zero, means do not care about rt
	// if MaxExpectedRT is not zero, means response's rt should no more than it
	MaxExpectedRT time.Duration
	// if MinExpectedRT is zero means do not care about it
	// if MinExpectedRT is not zero, means response's rt should more than it
	MinExpectedRT time.Duration
}

func (c *VerifyConfig) Verify(resp *Response) bool {
	if c == nil {
		return true
	}
	if resp.StatusCode != c.ExpectedStatusCode {
		log.DefaultLogger.Errorf("status is not expected: %d, %d", resp.StatusCode, c.ExpectedStatusCode)
		return false
	}
	for k, vs := range c.ExpectedHeader {
		k = strings.Title(k)
		values, ok := resp.Header[k]
		if !ok {
			log.DefaultLogger.Errorf("header key %s is not expected, got: %v", k, ok)
			return false
		}
		if !utils.SliceEqual(vs, values) {
			log.DefaultLogger.Errorf("header key %s is not expected, want: %v, got: %v", k, vs, values)
			return false
		}
	}
	//  if ExpectedBody is not nil, but length is zero, means expected empty content
	if c.ExpectedBody != nil {
		if !bytes.Equal(c.ExpectedBody, resp.Body) {
			log.DefaultLogger.Errorf("body is not expected: %v, %v", c.ExpectedBody, resp.Body)
			return false
		}
	}
	if c.MaxExpectedRT > 0 {
		if resp.Cost > c.MaxExpectedRT {
			log.DefaultLogger.Errorf("rt is %s, max expected is %s", resp.Cost, c.MaxExpectedRT)
			return false
		}
	}
	if c.MinExpectedRT > 0 {
		if resp.Cost > c.MinExpectedRT {
			log.DefaultLogger.Errorf("rt is %s, min expected is %s", resp.Cost, c.MaxExpectedRT)
			return false
		}
	}
	return true
}
