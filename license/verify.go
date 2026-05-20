package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/pem"
	"time"

	"github.com/oioio-space/maldev/license/canonical"
)

// Verify parses, authenticates, and authorises a license. The single returned
// error type is ErrLicenseInvalid; the detailed cause is logged via the
// injected slog.Logger.
func Verify(data []byte, trusted Trusted, opts ...VerifyOption) (*Verified, error) {
	state := newVerifyState(opts)

	// 1. Format + bound size.
	if len(data) == 0 || len(data) > MaxLicenseSize {
		return nil, state.fail(causeBadFormat)
	}
	blk, _ := pem.Decode(data)
	if blk == nil || blk.Type != pemLicense {
		return nil, state.fail(causeBadFormat)
	}
	raw, err := base64.StdEncoding.DecodeString(string(blk.Bytes))
	if err != nil {
		return nil, state.fail(causeBadFormat)
	}
	var w signedLicense
	if err := jsonUnmarshalStrict(raw, &w); err != nil {
		return nil, state.fail(causeBadFormat)
	}
	if w.License.Version != 1 {
		return nil, state.fail(causeBadFormat)
	}

	// 2. Key resolution.
	pub, ok := trusted.Lookup(w.KeyID)
	if !ok || w.KeyID != w.License.KeyID {
		return nil, state.fail(causeUnknownKey)
	}

	// 3. Signature.
	body, err := canonical.Marshal(w.License)
	if err != nil {
		return nil, state.fail(causeBadFormat)
	}
	if !ed25519.Verify(pub, signPayload(tagLicenseV1, body), w.Signature) {
		return nil, state.fail(causeBadSignature)
	}

	// 4. State file — wired in a later task.

	// 5. Time.
	now := state.clock.Now()
	skew := state.maxClockSkew
	if !w.License.NotBefore.IsZero() && w.License.NotBefore.After(now.Add(skew)) {
		return nil, state.fail(causeNotYetValid)
	}
	if !w.License.NotAfter.IsZero() && w.License.NotAfter.Before(now.Add(-skew)) {
		return nil, state.fail(causeExpired)
	}

	// 6. Audience / Issuer.
	if len(state.audience) > 0 && !audienceIntersects(state.audience, w.License.Audience) {
		if len(w.License.Audience) > 0 {
			return nil, state.fail(causeAudienceMismatch)
		}
		// Empty audience in the license = wildcard; tolerated.
	}
	if state.issuer != "" && state.issuer != w.License.Issuer {
		return nil, state.fail(causeIssuerMismatch)
	}

	// 7. Bindings.
	if c := checkBindings(w.License, state); c != causeOK {
		return nil, state.fail(c)
	}

	// 8-12. Wired in later tasks.

	return &Verified{
		License:  w.License,
		Payload:  []byte(w.License.Payload),
		KeyUsed:  w.KeyID,
		Warnings: state.warnings,
	}, nil
}

func audienceIntersects(want, have []string) bool {
	if len(have) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(have))
	for _, h := range have {
		set[h] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; ok {
			return true
		}
	}
	return false
}

func (s *verifyState) fail(c cause) error {
	s.logger.Warn("license verify failed", "cause", c.String())
	return invalid(c)
}

var _ = time.Second // import retention for future time-based steps
