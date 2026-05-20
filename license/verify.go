package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/pem"
	"time"

	"github.com/oioio-space/maldev/license/canonical"
	"github.com/oioio-space/maldev/license/heartbeat"
	"github.com/oioio-space/maldev/license/ntp"
	"github.com/oioio-space/maldev/license/revoke"
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

	// 4. State file: read if configured, detect rollback.
	var (
		st       State
		stateKey []byte
	)
	if state.statePath != "" {
		var hostFP []byte
		if state.stateHostIDFn != nil {
			hostFP, _ = state.stateHostIDFn()
		}
		if hostFP == nil {
			hostFP = make([]byte, 32)
		}
		stateKey = deriveStateKey(w.Signature, hostFP)
		if loaded, err := readState(state.statePath, stateKey); err == nil {
			st = loaded
		} else {
			state.logger.Warn("license state unreadable; resetting", "err", err)
		}
	}

	// 5. Time.
	now := state.clock.Now()
	skew := state.maxClockSkew
	floor := maxTime(st.TrustedFloor, st.LastSeenLocal)
	if !floor.IsZero() && now.Before(floor.Add(-state.maxClockSkew)) {
		return nil, state.fail(causeClockRollback)
	}

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

	// 8. Binary / Identity pinning.
	if c := checkPinning(w.License, state); c != causeOK {
		return nil, state.fail(c)
	}

	// 9. Revocation.
	if state.revokeSource != nil {
		list, fetched, ferr := loadOrFetchRevocation(state, pub, w.License.KeyID, now)
		if ferr != nil {
			if !st.LastFetchOk.IsZero() && now.Sub(st.LastFetchOk) > state.gracePeriod {
				return nil, state.fail(causeRevocationStale)
			}
			if st.LastFetchOk.IsZero() && state.gracePeriod == 0 {
				return nil, state.fail(causeRevocationStale)
			}
			state.logger.Warn("revocation fetch failed; using grace", "err", ferr)
		} else if list != nil {
			if list.IsRevoked(w.License.ID) {
				return nil, state.fail(causeRevoked)
			}
			if fetched {
				st.LastFetchOk = now
				if list.ServerTime.After(st.TrustedFloor) {
					st.TrustedFloor = list.ServerTime
				}
				if list.Sequence > st.LastSeenSequence {
					st.LastSeenSequence = list.Sequence
				}
			}
		}
	}

	// 10. Heartbeat. Skip the ping if a successful one happened within
	// heartbeatInterval — turns the option into a rate-limit rather than a
	// per-Verify round-trip.
	if state.heartbeatClient != nil &&
		(state.heartbeatInterval <= 0 || now.Sub(st.LastHeartbeatOk) >= state.heartbeatInterval) {
		nonce := make([]byte, 16)
		_, _ = rand.Read(nonce)
		reply, raw, herr := state.heartbeatClient.Ping(state.ctx, w.License.ID, nonce)
		if herr != nil {
			if state.gracePeriod == 0 || (now.Sub(st.LastHeartbeatOk) > state.gracePeriod) {
				return nil, state.fail(causeHeartbeatFailed)
			}
			state.logger.Warn("heartbeat fetch failed; using grace", "err", herr)
		} else {
			if _, vErr := heartbeat.VerifyReply(raw, pub, w.License.KeyID); vErr != nil {
				return nil, state.fail(causeHeartbeatFailed)
			}
			if subtle.ConstantTimeCompare(reply.NonceEcho, nonce) != 1 {
				return nil, state.fail(causeHeartbeatFailed)
			}
			if !reply.Ok {
				return nil, state.fail(causeHeartbeatFailed)
			}
			st.LastHeartbeatOk = now
			if reply.ServerTime.After(st.TrustedFloor) {
				st.TrustedFloor = reply.ServerTime
			}
		}
	}

	// 11. NTP cross-check.
	if state.ntpServer != "" {
		serverT, _, err := ntp.Query(state.ntpServer, 3*time.Second)
		if err == nil {
			drift := now.Sub(serverT)
			if drift < 0 {
				drift = -drift
			}
			if drift > state.ntpMaxDrift {
				if state.ntpStrict {
					return nil, state.fail(causeClockRollback)
				}
				state.warnings = append(state.warnings, "ntp drift exceeds threshold")
			}
		} else {
			state.warnings = append(state.warnings, "ntp query failed")
		}
	}

	// 12. Persist state.
	if state.statePath != "" && stateKey != nil {
		st.LastSeenLocal = maxTime(st.LastSeenLocal, now)
		if err := writeState(state.statePath, stateKey, st); err != nil {
			state.logger.Warn("license state write failed", "err", err)
		}
	}

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

func loadOrFetchRevocation(s *verifyState, pub ed25519.PublicKey, kid string, now time.Time) (*revoke.List, bool, error) {
	if s.revokeCachePath != "" {
		if l, err := revoke.LoadCache(s.revokeCachePath, pub, kid, now); err == nil {
			return l, false, nil
		}
	}
	raw, err := s.revokeSource.Fetch(s.ctx)
	if err != nil {
		return nil, false, err
	}
	l, err := revoke.VerifyBytes(raw, pub, kid)
	if err != nil {
		return nil, false, err
	}
	if s.revokeCachePath != "" {
		_ = revoke.StoreCache(s.revokeCachePath, raw, l.Sequence)
	}
	return l, true, nil
}
