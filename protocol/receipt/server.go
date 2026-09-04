package receipt

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"strings"

	"github.com/fil-forge/ucantone/ipld/codec/dagcbor"
	"github.com/fil-forge/ucantone/ucan"
	"github.com/fil-forge/ucantone/ucan/container"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("receipt")

type Finder interface {
	// FindByTask returns all UCAN containers that have tokens related to the
	// given task i.e. invocations or receipts for the task.
	FindByTask(ctx context.Context, task cid.Cid) iter.Seq2[ucan.Container, error]
}

// NewHandler creates a new [http.Handler] that serves receipt containers. It
// expects the URL path to end with the task CID, e.g. /receipt/<task-cid>
func NewHandler(tokens Finder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSuffix(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 {
			http.Error(w, "missing task CID", http.StatusBadRequest)
			return
		}
		task, err := cid.Parse(parts[len(parts)-1])
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid task CID: %v", err), http.StatusBadRequest)
			return
		}

		var invocations []ucan.Invocation
		var delegations []ucan.Delegation
		var receipts []ucan.Receipt
		for c, err := range tokens.FindByTask(r.Context(), task) {
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to find receipt token: %v", err), http.StatusInternalServerError)
				return
			}
			invocations = append(invocations, c.Invocations()...)
			delegations = append(delegations, c.Delegations()...)
			receipts = append(receipts, c.Receipts()...)
		}

		out := container.New(
			container.WithInvocations(invocations...),
			container.WithDelegations(delegations...),
			container.WithReceipts(receipts...),
		)

		if _, ok := out.Receipt(task); !ok {
			http.Error(w, "receipt not found for task", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", dagcbor.ContentType)
		err = out.MarshalCBOR(w)
		if err != nil {
			log.Errorw("marshaling receipt container", "error", err)
		}
	})
}
