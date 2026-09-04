package blobindexlookup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fil-forge/forge/indexing-service/pkg/types"
	"github.com/fil-forge/forge/protocol/blobindex"
	"github.com/fil-forge/forge/protocol/retrieval"
	"github.com/fil-forge/ucantone/execution"
	"github.com/fil-forge/ucantone/ucan/invocation"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipni/go-libipni/find/model"
)

var log = logging.Logger("blobindexlookup")

type simpleLookup struct {
	httpClient *http.Client
}

var _ BlobIndexLookup = (*simpleLookup)(nil)

func NewBlobIndexLookup(httpClient *http.Client) BlobIndexLookup {
	return &simpleLookup{httpClient}
}

// Find fetches the blob index from the given fetchURL
func (s *simpleLookup) Find(ctx context.Context, _ types.EncodedContextID, result model.ProviderResult, request types.RetrievalRequest) (blobindex.ShardedDagIndex, error) {
	var body io.ReadCloser
	// Make a UCAN authorized retrieval request.
	body, err := doAuthorizedRetrieval(ctx, s.httpClient, request)
	if err != nil {
		return nil, fmt.Errorf("executing authorized retrieval: %w", err)
	}
	defer body.Close()
	return blobindex.Extract(body)
}

func doAuthorizedRetrieval(ctx context.Context, httpClient *http.Client, request types.RetrievalRequest) (io.ReadCloser, error) {
	headers := http.Header{}
	if request.Range != nil {
		headers.Set("Range", fmt.Sprintf("bytes=%d-%d", request.Range.Start, request.Range.End))
	}

	log.Infow("authorized retrieval target",
		"url", request.URL.String(),
		"url_host", request.URL.Host,
		"url_path", request.URL.Path,
		"range", headers.Get("Range"),
		"audience", request.Auth.Audience.String(),
		"command", request.Auth.Command.String(),
		"subject", request.Auth.Subject.String(),
		"issuer", request.Auth.Issuer.DID().String(),
	)

	c, err := retrieval.NewClient(
		&request.URL,
		retrieval.WithHTTPClient(httpClient),
		retrieval.WithHTTPHeaders(headers),
	)
	if err != nil {
		return nil, err
	}

	prfLinks := make([]cid.Cid, 0, len(request.Auth.Proofs))
	for _, p := range request.Auth.Proofs {
		prfLinks = append(prfLinks, p.Link())
	}

	inv, err := invocation.Invoke(
		request.Auth.Issuer,
		request.Auth.Subject,
		request.Auth.Command,
		request.Auth.Arguments,
		invocation.WithProofs(prfLinks...),
		invocation.WithAudience(request.Auth.Audience),
	)
	if err != nil {
		return nil, err
	}

	res, err := c.Execute(execution.NewRequest(ctx, inv, execution.WithDelegations(request.Auth.Proofs...)))
	if err != nil {
		return nil, fmt.Errorf("executing retrieval invocation: %w", err)
	}

	if log.LevelEnabled(logging.LevelDebug) {
		// Diagnostic: dispatcher failures (InvalidAudience, HandlerNotFound,
		// validation errors) are encoded as failure receipts with no metadata.
		// The retrieval server then falls through to a default 200/empty/UCAN
		// response, leaving the failure invisible at the HTTP layer.
		if rcpt := res.Receipt(); rcpt != nil {
			log.Debugw("retrieval receipt outcome",
				"is_ok", rcpt.Out().IsOK(),
				"is_err", rcpt.Out().IsErr(),
				"ran", rcpt.Ran().String(),
			)
			if rcpt.Out().IsErr() {
				_, errBytes := rcpt.Out().Unpack()
				log.Debugw("retrieval receipt failure",
					"err_bytes_hex", fmt.Sprintf("%x", errBytes),
					"err_bytes_str", string(errBytes),
				)
			}
		}
	}

	meta, ok := res.Metadata().(*retrieval.HTTPHeaderResponseContainer)
	if !ok {
		return nil, errors.New("unexpected response metadata type")
	}
	if meta.StatusCode != http.StatusOK && meta.StatusCode != http.StatusPartialContent {
		defer meta.Body.Close()
		body, err := io.ReadAll(meta.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		return nil, fmt.Errorf("retrieval failed with status code: %d, body: %s", meta.StatusCode, string(body))
	}
	if log.LevelEnabled(logging.LevelDebug) {
		// Diagnostic: confirm what we actually got back. Buffer the body
		// so we can log a preview and still hand it to Extract.
		all, readErr := io.ReadAll(meta.Body)
		meta.Body.Close()
		preview := all
		if len(preview) > 64 {
			preview = preview[:64]
		}
		headerDump := make(map[string][]string, len(meta.Header))
		for k, v := range meta.Header {
			headerDump[k] = v
		}
		log.Debugw("authorized retrieval response",
			"status", meta.StatusCode,
			"body_len", len(all),
			"body_preview_hex", fmt.Sprintf("%x", preview),
			"read_err", readErr,
			"headers", headerDump,
		)
		return io.NopCloser(bytes.NewReader(all)), nil
	}
	return meta.Body, nil
}
