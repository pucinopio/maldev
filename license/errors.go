package license

import "errors"

// ErrLicenseInvalid is the single public error returned by Verify and related
// entry points. Detailed causes are logged via the injected slog.Logger but
// never surfaced in the error string — this prevents an attacker from
// brute-forcing constraint by constraint and observing which check failed.
var ErrLicenseInvalid = errors.New("license: verification failed")

type cause int

const (
	causeOK cause = iota
	causeBadFormat
	causeBadSignature
	causeUnknownKey
	causeNotYetValid
	causeExpired
	causeClockRollback
	causeAudienceMismatch
	causeIssuerMismatch
	causeBindingMachineMismatch
	causeBindingPasswordMismatch
	causeBindingTOTPMismatch
	causeBindingCustomMismatch
	causeBinaryHashMismatch
	causeIdentityMismatch
	causeRevoked
	causeRevocationStale
	causeHeartbeatFailed
	causeStateCorrupted
)

func (c cause) String() string {
	switch c {
	case causeBadFormat:
		return "bad-format"
	case causeBadSignature:
		return "bad-signature"
	case causeUnknownKey:
		return "unknown-key"
	case causeNotYetValid:
		return "not-yet-valid"
	case causeExpired:
		return "expired"
	case causeClockRollback:
		return "clock-rollback"
	case causeAudienceMismatch:
		return "audience-mismatch"
	case causeIssuerMismatch:
		return "issuer-mismatch"
	case causeBindingMachineMismatch:
		return "binding-machine-mismatch"
	case causeBindingPasswordMismatch:
		return "binding-password-mismatch"
	case causeBindingTOTPMismatch:
		return "binding-totp-mismatch"
	case causeBindingCustomMismatch:
		return "binding-custom-mismatch"
	case causeBinaryHashMismatch:
		return "binary-hash-mismatch"
	case causeIdentityMismatch:
		return "identity-mismatch"
	case causeRevoked:
		return "revoked"
	case causeRevocationStale:
		return "revocation-stale"
	case causeHeartbeatFailed:
		return "heartbeat-failed"
	case causeStateCorrupted:
		return "state-corrupted"
	default:
		return "unknown"
	}
}

// invalid returns the opaque public sentinel. The cause is intentionally not
// embedded in the returned error: an attacker reading err.Error() must not
// learn which check failed. Causes are routed to slog separately via
// verifyState.fail.
func invalid(_ cause) error { return ErrLicenseInvalid }
