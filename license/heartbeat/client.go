package heartbeat

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/oioio-space/maldev/license/canonical"
)

const pemHeartbeatBlock = "MALDEV HEARTBEAT REPLY"

type signedReply struct {
	Reply     Reply  `json:"rep"`
	Signature []byte `json:"sig"`
	KeyID     string `json:"kid"`
}

func SignReply(r Reply, priv ed25519.PrivateKey) ([]byte, error) {
	if r.Version == 0 {
		r.Version = 1
	}
	body, err := canonical.Marshal(r)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, append([]byte(tagHeartbeatV1), body...))
	wrapped, err := canonical.Marshal(signedReply{Reply: r, Signature: sig, KeyID: r.KeyID})
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  pemHeartbeatBlock,
		Bytes: []byte(base64.StdEncoding.EncodeToString(wrapped)),
	}), nil
}

func VerifyReply(data []byte, pub ed25519.PublicKey, expectedKID string) (*Reply, error) {
	blk, _ := pem.Decode(data)
	if blk == nil || blk.Type != pemHeartbeatBlock {
		return nil, errors.New("heartbeat: not a heartbeat PEM")
	}
	raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
	if err != nil {
		return nil, err
	}
	var w signedReply
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	if expectedKID != "" && w.KeyID != expectedKID {
		return nil, errors.New("heartbeat: kid mismatch")
	}
	body, err := canonical.Marshal(w.Reply)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(pub, append([]byte(tagHeartbeatV1), body...), w.Signature) {
		return nil, errors.New("heartbeat: signature invalid")
	}
	return &w.Reply, nil
}

func HTTPClient(url string, client *http.Client) Client {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpClient{url: url, c: client}
}

type httpClient struct {
	url string
	c   *http.Client
}

func (h *httpClient) Ping(ctx context.Context, licenseID string, nonce []byte) (Reply, []byte, error) {
	reqBody, err := json.Marshal(Request{LicenseID: licenseID, Nonce: nonce})
	if err != nil {
		return Reply{}, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(reqBody))
	if err != nil {
		return Reply{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.c.Do(req)
	if err != nil {
		return Reply{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return Reply{}, nil, fmt.Errorf("heartbeat: HTTP %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return Reply{}, nil, err
	}
	blk, _ := pem.Decode(raw)
	if blk == nil {
		return Reply{}, nil, errors.New("heartbeat: bad PEM in reply")
	}
	inner, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
	if err != nil {
		return Reply{}, nil, err
	}
	var w signedReply
	if err := json.Unmarshal(inner, &w); err != nil {
		return Reply{}, nil, err
	}
	return w.Reply, raw, nil
}
