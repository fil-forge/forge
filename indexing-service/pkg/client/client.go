package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/fil-forge/forge/indexing-service/pkg/service/queryresult"
	"github.com/fil-forge/forge/indexing-service/pkg/types"
	assertcaps "github.com/fil-forge/forge/protocol/commands/assert"
	claimcaps "github.com/fil-forge/forge/protocol/commands/claim"
	"github.com/fil-forge/forge/protocol/digestutil"
	"github.com/fil-forge/forge/protocol/retrieval"
	"github.com/fil-forge/ucantone/client"
	"github.com/fil-forge/ucantone/did"
	edm "github.com/fil-forge/ucantone/errors/datamodel"
	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/fil-forge/ucantone/ucan/invocation"
	cbg "github.com/whyrusleeping/cbor-gen"
)

const claimsPath = "/claims"

var ErrNoReceiptFound = errors.New("missing receipt link")

type ErrFailedResponse struct {
	StatusCode int
	Body       string
}

func errFromResponse(res *http.Response) ErrFailedResponse {
	err := ErrFailedResponse{StatusCode: res.StatusCode}

	message, merr := io.ReadAll(res.Body)
	if merr != nil {
		err.Body = merr.Error()
	} else {
		err.Body = string(message)
	}
	return err
}

func (e ErrFailedResponse) Error() string {
	return fmt.Sprintf("http request failed, status: %d %s, message: %s", e.StatusCode, http.StatusText(e.StatusCode), e.Body)
}

type Client struct {
	servicePrincipal did.DID
	serviceURL       url.URL
	client           *client.HTTPClient
	httpClient       *http.Client
	telemetryEnabled bool
}

// execute sends the given invocation using the provided executor and decodes
// the response into the specified type.
func execute[T cbg.CBORUnmarshaler](
	ctx context.Context,
	executor execution.Executor,
	inv ucan.Invocation,
	options ...execution.RequestOption,
) (T, ucan.Receipt, error) {
	var zero T
	resp, err := executor.Execute(execution.NewRequest(ctx, inv, options...))
	if err != nil {
		return zero, nil, fmt.Errorf("executing invocation: %w", err)
	}

	rcpt := resp.Receipt()

	o, x := rcpt.Out().Unpack()
	if rcpt.Out().IsErr() {
		var model edm.ErrorModel
		if err := model.UnmarshalCBOR(bytes.NewReader(x)); err != nil {
			return zero, nil, fmt.Errorf("executing invocation")
		}
		return zero, nil, fmt.Errorf("executing invocation: %w", model)
	}

	// if ok is a pointer type, allocate the underlying value so
	// UnmarshalCBOR has a non-nil pointer to write into.
	var ok T
	typ := reflect.TypeOf(ok)
	if typ.Kind() == reflect.Ptr {
		ok = reflect.New(typ.Elem()).Interface().(T)
	}
	if err := ok.UnmarshalCBOR(bytes.NewReader(o)); err != nil {
		return zero, nil, fmt.Errorf("unmarshaling invocation response: %w", err)
	}
	return ok, rcpt, nil
}

func (c *Client) PublishIndexClaim(ctx context.Context, issuer ucan.Issuer, args assertcaps.IndexArguments, options ...invocation.Option) error {
	inv, err := assertcaps.Index.Invoke(issuer, c.servicePrincipal, &args, options...)
	if err != nil {
		return fmt.Errorf("generating invocation: %w", err)
	}
	_, _, err = execute[*assertcaps.IndexOK](ctx, c.client, inv)
	return err
}

func (c *Client) PublishEqualsClaim(ctx context.Context, issuer ucan.Issuer, args assertcaps.EqualsArguments, options ...invocation.Option) error {
	inv, err := assertcaps.Equals.Invoke(issuer, c.servicePrincipal, &args, options...)
	if err != nil {
		return fmt.Errorf("generating invocation: %w", err)
	}
	_, _, err = execute[*assertcaps.EqualsOK](ctx, c.client, inv)
	return err
}

func (c *Client) CacheClaim(ctx context.Context, issuer ucan.Issuer, cacheClaim ucan.Invocation, provider claimcaps.Provider, options ...invocation.Option) error {
	inv, err := claimcaps.Cache.Invoke(issuer, c.servicePrincipal, &claimcaps.CacheArguments{
		Claim:    cacheClaim.Link(),
		Provider: provider,
	}, options...)
	if err != nil {
		return fmt.Errorf("generating invocation: %w", err)
	}
	_, _, err = execute[*claimcaps.CacheOK](ctx, c.client, inv, execution.WithInvocations(cacheClaim))
	return err
}

func (c *Client) QueryClaims(ctx context.Context, query types.Query) (types.QueryResult, error) {
	var span trace.Span
	if c.telemetryEnabled {
		tracer := otel.Tracer("client")
		ctx, span = tracer.Start(ctx, "client.QueryClaims",
			trace.WithSpanKind(trace.SpanKindClient),
		)
		defer span.End()
	}

	url := c.serviceURL.JoinPath(claimsPath)
	q := url.Query()
	q.Add("type", query.Type.String())
	for _, mh := range query.Hashes {
		q.Add("multihash", digestutil.Format(mh))
	}
	for _, space := range query.Match.Subject {
		q.Add("spaces", space.String())
	}
	url.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "creating request")
		}
		return nil, fmt.Errorf("creating request: %w", err)
	}
	if c.telemetryEnabled {
		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	}

	// If there are query delegations, then add them to an X-UCAN-Container header.
	if len(query.Delegations) > 0 {
		msg := container.New(container.WithDelegations(query.Delegations...))
		headerValue, err := container.Encode(container.Base64urlGzip, msg)
		if err != nil {
			return nil, fmt.Errorf("encoding %q header: %w", retrieval.HTTPHeaderName, err)
		}
		req.Header.Set(retrieval.HTTPHeaderName, string(headerValue))
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "sending query")
		}
		return nil, fmt.Errorf("sending query to server: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		if span != nil {
			span.RecordError(errFromResponse(res))
			span.SetStatus(codes.Error, "non-2xx response")
		}
		return nil, errFromResponse(res)
	}

	return queryresult.Extract(res.Body)
}

type Option func(*Client)

// WithHTTPClient configures the HTTP client to use for making query requests
// and invocations.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTelemetryEnabled toggles client-side tracing and context propagation.
func WithTelemetryEnabled(enabled bool) Option {
	return func(c *Client) {
		c.telemetryEnabled = enabled
	}
}

func New(servicePrincipal did.DID, serviceURL url.URL, options ...Option) (*Client, error) {
	c := Client{
		servicePrincipal: servicePrincipal,
		serviceURL:       serviceURL,
		httpClient:       &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range options {
		opt(&c)
	}
	clientOpts := []client.HTTPOption{}
	if c.httpClient != nil {
		clientOpts = append(clientOpts, client.WithHTTPClient(c.httpClient))
	}
	client, err := client.NewHTTP(&serviceURL, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating HTTP client: %w", err)
	}
	c.client = client
	return &c, nil
}
