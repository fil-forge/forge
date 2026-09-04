package retrieval

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/fil-forge/ucantone/transport"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
)

var (
	DefaultHTTPHeaderInboundCodec  = &HTTPHeaderInboundCodec{}
	DefaultHTTPHeaderOutboundCodec = &HTTPHeaderOutboundCodec{}
	HTTPHeaderName                 = "X-UCAN-Container"
)

type HTTPHeaderRequestContainer struct {
	ucan.Container
	Method string
	URL    *url.URL
	Header http.Header
	Body   io.ReadCloser
}

type HTTPHeaderResponseContainer struct {
	ucan.Container
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

type HTTPHeaderInboundCodec struct{}

var _ transport.InboundCodec[*http.Request, *http.Response] = (*HTTPHeaderInboundCodec)(nil)

func (h *HTTPHeaderInboundCodec) Decode(r *http.Request) (ucan.Container, error) {
	if r.Header.Get(HTTPHeaderName) == "" {
		return nil, fmt.Errorf("missing required %q header", HTTPHeaderName)
	}
	ct, err := container.Decode([]byte(r.Header.Get(HTTPHeaderName)))
	if err != nil {
		return nil, fmt.Errorf("decoding container: %w", err)
	}
	return &HTTPHeaderRequestContainer{
		Container: ct,
		Method:    r.Method,
		URL:       r.URL,
		Header:    r.Header,
		Body:      r.Body,
	}, nil
}

func (h *HTTPHeaderInboundCodec) Encode(c ucan.Container) (*http.Response, error) {
	status := http.StatusOK
	headers := http.Header{}
	var body io.ReadCloser
	if hc, ok := c.(*HTTPHeaderResponseContainer); ok {
		if hc.StatusCode != 0 {
			status = hc.StatusCode
		}
		if hc.Header != nil {
			headers = hc.Header
		}
		body = hc.Body
	}
	resp := &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       body,
	}
	ctBytes, err := container.Encode(container.Base64Gzip, c)
	if err != nil {
		return nil, fmt.Errorf("encoding container: %w", err)
	}
	resp.Header.Set(HTTPHeaderName, string(ctBytes))
	// ensure the Vary header is set for ALL responses
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Vary
	resp.Header.Add("Vary", HTTPHeaderName)
	return resp, nil
}

type HTTPResponseContainer struct {
	ucan.Container
	Response *http.Response
}

type HTTPHeaderOutboundCodec struct{}

var _ transport.OutboundCodec[*http.Request, *http.Response] = (*HTTPHeaderOutboundCodec)(nil)

func (h *HTTPHeaderOutboundCodec) Encode(c ucan.Container) (*http.Request, error) {
	method := http.MethodGet
	headers := http.Header{}
	var body io.ReadCloser
	if hc, ok := c.(*HTTPHeaderRequestContainer); ok {
		if hc.Method != "" {
			method = hc.Method
		}
		if hc.Header != nil {
			headers = hc.Header
		}
		body = hc.Body
	}
	req := &http.Request{
		Method: method,
		Body:   body,
		Header: headers,
	}
	ctBytes, err := container.Encode(container.Base64Gzip, c)
	if err != nil {
		return nil, fmt.Errorf("encoding container: %w", err)
	}
	req.Header.Set(HTTPHeaderName, string(ctBytes))
	return req, nil
}

func (h *HTTPHeaderOutboundCodec) Decode(r *http.Response) (ucan.Container, error) {
	if r.Header.Get(HTTPHeaderName) == "" {
		return nil, fmt.Errorf("missing required %q header", HTTPHeaderName)
	}
	ct, err := container.Decode([]byte(r.Header.Get(HTTPHeaderName)))
	if err != nil {
		return nil, fmt.Errorf("decoding container: %w", err)
	}
	hct := HTTPHeaderResponseContainer{
		Container:  ct,
		StatusCode: r.StatusCode,
		Header:     r.Header,
		Body:       r.Body,
	}
	return &hct, nil
}
