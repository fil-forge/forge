//go:build !codegen

package s3

import (
	"fmt"
	"io"
	"sort"

	jsg "github.com/alanshaw/dag-json-gen"
	"github.com/fil-forge/ucantone/did"
	cid "github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

// These codecs are hand-written because cbor-gen / dag-json-gen do not support
// slice-valued maps.

const (
	maxString = 8192
	maxLen    = 4096
)

// --- shared CBOR helpers ---

func writeCborStringField(cw *cbg.CborWriter, s string) error {
	if len(s) > maxString {
		return xerrors.Errorf("string value was too long (%d)", len(s))
	}
	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(s))); err != nil {
		return err
	}
	_, err := cw.WriteString(s)
	return err
}

func readCborStringField(cr *cbg.CborReader) (string, error) {
	return cbg.ReadStringWithMax(cr, maxString)
}

func writeCborArrayHeader(cw *cbg.CborWriter, n int) error {
	if n > maxString {
		return xerrors.Errorf("slice value was too long (%d)", n)
	}
	return cw.WriteMajorTypeHeader(cbg.MajArray, uint64(n))
}

func readCborArrayHeader(cr *cbg.CborReader) (uint64, error) {
	maj, extra, err := cr.ReadHeader()
	if err != nil {
		return 0, err
	}
	if maj != cbg.MajArray {
		return 0, fmt.Errorf("expected cbor array")
	}
	if extra > maxString {
		return 0, fmt.Errorf("array too large (%d)", extra)
	}
	return extra, nil
}

func writeCborMapHeader(cw *cbg.CborWriter, n int) error {
	if n > maxLen {
		return xerrors.Errorf("map too large (%d)", n)
	}
	return cw.WriteMajorTypeHeader(cbg.MajMap, uint64(n))
}

func readCborMapHeader(cr *cbg.CborReader) (uint64, error) {
	maj, extra, err := cr.ReadHeader()
	if err != nil {
		return 0, err
	}
	if maj != cbg.MajMap {
		return 0, fmt.Errorf("expected a map (major type 5)")
	}
	if extra > maxLen {
		return 0, fmt.Errorf("map too large (%d)", extra)
	}
	return extra, nil
}

// sortedDIDs returns the DID keys of m sorted by their string encoding, so the
// encoded map keys come out in the same (lexicographic) order the generators
// use for string-keyed maps.
func sortedDIDs[V any](m map[did.DID]V) []did.DID {
	keys := make([]did.DID, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	return keys
}

// sortedCIDs returns the CID keys of m sorted by their string encoding.
func sortedCIDs[V any](m map[cid.Cid]V) []cid.Cid {
	keys := make([]cid.Cid, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	return keys
}

// --- PermissionSet: map[did.DID][]string ---

func (t PermissionSet) MarshalCBOR(w io.Writer) error {
	cw := cbg.NewCborWriter(w)
	if err := writeCborMapHeader(cw, len(t.Entries)); err != nil {
		return err
	}
	for _, k := range sortedDIDs(t.Entries) {
		if err := writeCborStringField(cw, k.String()); err != nil {
			return err
		}
		perms := t.Entries[k]
		if err := writeCborArrayHeader(cw, len(perms)); err != nil {
			return err
		}
		for _, p := range perms {
			if err := writeCborStringField(cw, p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *PermissionSet) UnmarshalCBOR(r io.Reader) error {
	cr := cbg.NewCborReader(r)
	n, err := readCborMapHeader(cr)
	if err != nil {
		return err
	}
	m := make(map[did.DID][]string, n)
	for i := uint64(0); i < n; i++ {
		ks, err := readCborStringField(cr)
		if err != nil {
			return err
		}
		k, err := did.Parse(ks)
		if err != nil {
			return xerrors.Errorf("parsing access key did %q: %w", ks, err)
		}
		alen, err := readCborArrayHeader(cr)
		if err != nil {
			return err
		}
		perms := make([]string, 0, alen)
		for j := uint64(0); j < alen; j++ {
			p, err := readCborStringField(cr)
			if err != nil {
				return err
			}
			perms = append(perms, p)
		}
		m[k] = perms
	}
	*t = PermissionSet{Entries: m}
	return nil
}

func (t PermissionSet) MarshalDagJSON(w io.Writer) error {
	jw := jsg.NewDagJsonWriter(w)
	if err := jw.WriteObjectOpen(); err != nil {
		return err
	}
	for i, k := range sortedDIDs(t.Entries) {
		if err := writeJSONKey(jw, k.String(), i > 0); err != nil {
			return err
		}
		if err := jw.WriteArrayOpen(); err != nil {
			return err
		}
		for j, p := range t.Entries[k] {
			if j > 0 {
				if err := jw.WriteComma(); err != nil {
					return err
				}
			}
			if err := jw.WriteString(p); err != nil {
				return err
			}
		}
		if err := jw.WriteArrayClose(); err != nil {
			return err
		}
	}
	return jw.WriteObjectClose()
}

func (t *PermissionSet) UnmarshalDagJSON(r io.Reader) error {
	jr := jsg.NewDagJsonReader(r)
	m := map[did.DID][]string{}
	err := readJSONObject(jr, func(ks string) error {
		k, err := did.Parse(ks)
		if err != nil {
			return xerrors.Errorf("parsing access key did %q: %w", ks, err)
		}
		perms, err := readJSONArray(jr, func() (string, error) { return jr.ReadString(maxString) })
		if err != nil {
			return err
		}
		m[k] = perms
		return nil
	})
	if err != nil {
		return err
	}
	*t = PermissionSet{Entries: m}
	return nil
}

// --- ProofSet: map[cid.Cid][]cid.Cid ---

func (t ProofSet) MarshalCBOR(w io.Writer) error {
	cw := cbg.NewCborWriter(w)
	if err := writeCborMapHeader(cw, len(t.Entries)); err != nil {
		return err
	}
	for _, k := range sortedCIDs(t.Entries) {
		if err := writeCborStringField(cw, k.String()); err != nil {
			return err
		}
		links := t.Entries[k]
		if err := writeCborArrayHeader(cw, len(links)); err != nil {
			return err
		}
		for _, c := range links {
			if err := cbg.WriteCid(cw, c); err != nil {
				return xerrors.Errorf("failed to write cid: %w", err)
			}
		}
	}
	return nil
}

func (t *ProofSet) UnmarshalCBOR(r io.Reader) error {
	cr := cbg.NewCborReader(r)
	n, err := readCborMapHeader(cr)
	if err != nil {
		return err
	}
	m := make(map[cid.Cid][]cid.Cid, n)
	for i := uint64(0); i < n; i++ {
		ks, err := readCborStringField(cr)
		if err != nil {
			return err
		}
		k, err := cid.Decode(ks)
		if err != nil {
			return xerrors.Errorf("parsing delegation cid %q: %w", ks, err)
		}
		alen, err := readCborArrayHeader(cr)
		if err != nil {
			return err
		}
		links := make([]cid.Cid, 0, alen)
		for j := uint64(0); j < alen; j++ {
			c, err := cbg.ReadCid(cr)
			if err != nil {
				return xerrors.Errorf("failed to read cid: %w", err)
			}
			links = append(links, c)
		}
		m[k] = links
	}
	*t = ProofSet{Entries: m}
	return nil
}

func (t ProofSet) MarshalDagJSON(w io.Writer) error {
	jw := jsg.NewDagJsonWriter(w)
	if err := jw.WriteObjectOpen(); err != nil {
		return err
	}
	for i, k := range sortedCIDs(t.Entries) {
		if err := writeJSONKey(jw, k.String(), i > 0); err != nil {
			return err
		}
		if err := jw.WriteArrayOpen(); err != nil {
			return err
		}
		for j, c := range t.Entries[k] {
			if j > 0 {
				if err := jw.WriteComma(); err != nil {
					return err
				}
			}
			if err := jw.WriteCid(c); err != nil {
				return err
			}
		}
		if err := jw.WriteArrayClose(); err != nil {
			return err
		}
	}
	return jw.WriteObjectClose()
}

func (t *ProofSet) UnmarshalDagJSON(r io.Reader) error {
	jr := jsg.NewDagJsonReader(r)
	m := map[cid.Cid][]cid.Cid{}
	err := readJSONObject(jr, func(ks string) error {
		k, err := cid.Decode(ks)
		if err != nil {
			return xerrors.Errorf("parsing delegation cid %q: %w", ks, err)
		}
		links, err := readJSONArray(jr, func() (cid.Cid, error) { return jr.ReadCid() })
		if err != nil {
			return err
		}
		m[k] = links
		return nil
	})
	if err != nil {
		return err
	}
	*t = ProofSet{Entries: m}
	return nil
}

// --- KeySet: map[did.DID][]VerificationKey ---

func (t KeySet) MarshalCBOR(w io.Writer) error {
	cw := cbg.NewCborWriter(w)
	if err := writeCborMapHeader(cw, len(t.Entries)); err != nil {
		return err
	}
	for _, k := range sortedDIDs(t.Entries) {
		if err := writeCborStringField(cw, k.String()); err != nil {
			return err
		}
		keys := t.Entries[k]
		if err := writeCborArrayHeader(cw, len(keys)); err != nil {
			return err
		}
		for i := range keys {
			if err := keys[i].MarshalCBOR(cw); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *KeySet) UnmarshalCBOR(r io.Reader) error {
	cr := cbg.NewCborReader(r)
	n, err := readCborMapHeader(cr)
	if err != nil {
		return err
	}
	m := make(map[did.DID][]VerificationKey, n)
	for i := uint64(0); i < n; i++ {
		ks, err := readCborStringField(cr)
		if err != nil {
			return err
		}
		k, err := did.Parse(ks)
		if err != nil {
			return xerrors.Errorf("parsing access key did %q: %w", ks, err)
		}
		alen, err := readCborArrayHeader(cr)
		if err != nil {
			return err
		}
		keys := make([]VerificationKey, alen)
		for j := uint64(0); j < alen; j++ {
			if err := keys[j].UnmarshalCBOR(cr); err != nil {
				return err
			}
		}
		m[k] = keys
	}
	*t = KeySet{Entries: m}
	return nil
}

func (t KeySet) MarshalDagJSON(w io.Writer) error {
	jw := jsg.NewDagJsonWriter(w)
	if err := jw.WriteObjectOpen(); err != nil {
		return err
	}
	for i, k := range sortedDIDs(t.Entries) {
		if err := writeJSONKey(jw, k.String(), i > 0); err != nil {
			return err
		}
		if err := jw.WriteArrayOpen(); err != nil {
			return err
		}
		keys := t.Entries[k]
		for j := range keys {
			if j > 0 {
				if err := jw.WriteComma(); err != nil {
					return err
				}
			}
			if err := keys[j].MarshalDagJSON(jw); err != nil {
				return err
			}
		}
		if err := jw.WriteArrayClose(); err != nil {
			return err
		}
	}
	return jw.WriteObjectClose()
}

func (t *KeySet) UnmarshalDagJSON(r io.Reader) error {
	jr := jsg.NewDagJsonReader(r)
	m := map[did.DID][]VerificationKey{}
	err := readJSONObject(jr, func(ks string) error {
		k, err := did.Parse(ks)
		if err != nil {
			return xerrors.Errorf("parsing access key did %q: %w", ks, err)
		}
		keys, err := readJSONArray(jr, func() (VerificationKey, error) {
			var vk VerificationKey
			err := vk.UnmarshalDagJSON(jr)
			return vk, err
		})
		if err != nil {
			return err
		}
		m[k] = keys
		return nil
	})
	if err != nil {
		return err
	}
	*t = KeySet{Entries: m}
	return nil
}

// --- shared DAG-JSON helpers ---

// writeJSONKey writes an object key, preceded by a comma when comma is true
// (i.e. when it is not the first entry of the object).
func writeJSONKey(jw *jsg.DagJsonWriter, name string, comma bool) error {
	if comma {
		if err := jw.WriteComma(); err != nil {
			return err
		}
	}
	if err := jw.WriteString(name); err != nil {
		return err
	}
	return jw.WriteObjectColon()
}

// readJSONObject reads a DAG-JSON object, invoking fn for each entry after
// consuming its key and colon. fn is responsible for reading the value.
func readJSONObject(jr *jsg.DagJsonReader, fn func(name string) error) (err error) {
	defer func() {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}()
	if err := jr.ReadObjectOpen(); err != nil {
		return err
	}
	close, err := jr.PeekObjectClose()
	if err != nil {
		return err
	}
	if close {
		return jr.ReadObjectClose()
	}
	for {
		name, err := jr.ReadString(maxString)
		if err != nil {
			return err
		}
		if err := jr.ReadObjectColon(); err != nil {
			return err
		}
		if err := fn(name); err != nil {
			return err
		}
		close, err := jr.ReadObjectCloseOrComma()
		if err != nil {
			return err
		}
		if close {
			return nil
		}
	}
}

// readJSONArray reads a DAG-JSON array, invoking read for each element.
func readJSONArray[T any](jr *jsg.DagJsonReader, read func() (T, error)) ([]T, error) {
	if err := jr.ReadArrayOpen(); err != nil {
		return nil, err
	}
	close, err := jr.PeekArrayClose()
	if err != nil {
		return nil, err
	}
	var out []T
	if close {
		return out, jr.ReadArrayClose()
	}
	for {
		v, err := read()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		close, err := jr.ReadArrayCloseOrComma()
		if err != nil {
			return nil, err
		}
		if close {
			return out, nil
		}
	}
}
