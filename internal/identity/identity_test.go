package identity_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/fil-forge/forge/internal/identity"
	"github.com/stretchr/testify/require"
)

func TestDIDDocument(t *testing.T) {
	const serviceDID = "did:web:example.com"

	id, err := identity.New("", serviceDID)
	require.NoError(t, err)

	doc, err := id.DIDDocument()
	require.NoError(t, err)

	docJSON, err := json.Marshal(doc)
	require.NoError(t, err)

	var parsed struct {
		ID                 string `json:"id"`
		VerificationMethod []struct {
			ID         string `json:"id"`
			Controller string `json:"controller"`
		} `json:"verificationMethod"`
	}
	require.NoError(t, json.Unmarshal(docJSON, &parsed))

	require.Equal(t, serviceDID, parsed.ID)
	require.Len(t, parsed.VerificationMethod, 1)

	vm := parsed.VerificationMethod[0]
	require.Equal(t, serviceDID+"#key-0", vm.ID)
	require.False(t, strings.Contains(vm.ID, "%23"), "verification method ID must not contain a percent-encoded '#'")
	require.Equal(t, serviceDID, vm.Controller)
}
