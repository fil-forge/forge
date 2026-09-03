package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"

	jsg "github.com/alanshaw/dag-json-gen"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var _ cbg.CBORMarshaler = (*CborURL)(nil)
var _ cbg.CBORUnmarshaler = (*CborURL)(nil)

type CborURL url.URL

func (cu CborURL) URL() *url.URL {
	u := url.URL(cu)
	return &u
}

func (cu CborURL) MarshalCBOR(w io.Writer) error {
	urlStr := cu.URL().String()

	if len(urlStr) > 8192 {
		return errors.New("value in field cu.URL was too long")
	}

	cw := cbg.NewCborWriter(w)

	if err := cw.WriteMajorTypeHeader(cbg.MajTextString, uint64(len(urlStr))); err != nil {
		return err
	}

	if _, err := cw.WriteString(urlStr); err != nil {
		return err
	}

	return nil
}

func (cu *CborURL) UnmarshalCBOR(r io.Reader) error {
	cr := cbg.NewCborReader(r)
	sval, err := cbg.ReadStringWithMax(cr, 8192)
	if err != nil {
		return err
	}

	parsed, err := url.Parse(sval)
	if err != nil {
		return err
	}

	*(*url.URL)(cu) = *parsed

	return nil
}

func (cu CborURL) MarshalDagJSON(w io.Writer) error {
	urlStr := cu.URL().String()
	if len(urlStr) > 8192 {
		return errors.New("value in field cu.URL was too long")
	}
	buf, err := json.Marshal(urlStr)
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

func (cu *CborURL) UnmarshalDagJSON(r io.Reader) error {
	var urlStr string
	jr := jsg.NewDagJsonReader(r)
	urlStr, err := jr.ReadString(8192)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	*(*url.URL)(cu) = *parsed
	return nil
}

func (cu CborURL) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	if err := cu.MarshalDagJSON(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (cu *CborURL) UnmarshalJSON(b []byte) error {
	return cu.UnmarshalDagJSON(bytes.NewReader(b))
}
