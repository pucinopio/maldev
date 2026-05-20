package license

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"

	"github.com/oioio-space/maldev/license/totp"
)

const (
	bindingMachine  = "machine"
	bindingPassword = "password"
	bindingTOTP     = "totp"
	bindingCustom   = "custom:" // prefix
)

// Default argon2id parameters chosen for ~100 ms on a 2024-era laptop CPU.
// Any BindPassword call without an explicit override uses these. The chosen
// values are also stamped into Binding.Params at issue time so a future
// retuning is a non-breaking change: licences carry their own parameters
// and Verify uses what the binding stores, not what the verifying binary
// happens to have hardcoded.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// DefaultArgon2idParams returns a copy of the package defaults. Use as a
// starting point when overriding with BindPasswordWithParams.
func DefaultArgon2idParams() BindingParams {
	return BindingParams{
		ArgonTime:    argonTime,
		ArgonMemory:  argonMemory,
		ArgonThreads: argonThreads,
		ArgonKeyLen:  argonKeyLen,
	}
}

// resolveArgonParams returns p with any zero field filled by the package
// defaults. Shared by BindPasswordWithParams (stamping at issue) and the
// verify path (reading from the binding) so the two cannot drift.
func resolveArgonParams(p BindingParams) BindingParams {
	if p.ArgonTime == 0 {
		p.ArgonTime = argonTime
	}
	if p.ArgonMemory == 0 {
		p.ArgonMemory = argonMemory
	}
	if p.ArgonThreads == 0 {
		p.ArgonThreads = argonThreads
	}
	if p.ArgonKeyLen == 0 {
		p.ArgonKeyLen = argonKeyLen
	}
	return p
}

func argonParamsFor(b Binding) BindingParams {
	if b.Params == nil {
		return DefaultArgon2idParams()
	}
	return resolveArgonParams(*b.Params)
}

// BindMachineIDs builds a binding accepting any of the listed machine ids.
func BindMachineIDs(ids ...string) Binding {
	return Binding{Type: bindingMachine, Value: append([]string(nil), ids...)}
}

// BindCustom builds a typed custom binding. Multiple values accept any-match.
func BindCustom(name string, values ...string) Binding {
	return Binding{Type: bindingCustom + name, Value: append([]string(nil), values...)}
}

// BindTOTP creates a binding requiring a current RFC 6238 TOTP code at Verify
// time. The base32-encoded secret is stored in the binding (signed but
// readable by anyone who holds the license — see docs for the security
// trade-off). Pair with WithTOTPCode at the verification site.
//
// Provision the user's authenticator app once with totp.QRImagePNG or
// totp.QRImageASCII using the same secret.
func BindTOTP(secret string) Binding {
	return Binding{Type: bindingTOTP, Value: []string{secret}}
}

// BindPassword derives argon2id(salt, password) with the package defaults
// and stamps those defaults into Binding.Params. Verification uses the
// stored parameters, not the verifier's current defaults, so the issuer can
// retune later without breaking existing licences.
func BindPassword(password string) (Binding, error) {
	return BindPasswordWithParams(password, DefaultArgon2idParams())
}

// BindPasswordWithParams is like BindPassword but lets the issuer override
// the argon2id parameters. Useful for stronger settings on long-lived
// licences or for downgrading the cost on resource-constrained verifiers.
func BindPasswordWithParams(password string, p BindingParams) (Binding, error) {
	if password == "" {
		return Binding{}, errors.New("license: empty password")
	}
	p = resolveArgonParams(p)
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return Binding{}, err
	}
	hash := argon2.IDKey([]byte(password), salt, p.ArgonTime, p.ArgonMemory, p.ArgonThreads, p.ArgonKeyLen)
	return Binding{Type: bindingPassword, Hash: hash, Salt: salt, Params: &p}, nil
}

// VerifierFunc lets callers register custom binding types. Return true to
// accept.
type VerifierFunc func(b Binding, s *verifyState) bool

var (
	verifierMu      sync.RWMutex
	globalVerifiers = map[string]VerifierFunc{}
)

// RegisterVerifier installs a callback for a custom binding type (without
// the "custom:" prefix). Safe from package init.
func RegisterVerifier(name string, fn VerifierFunc) {
	verifierMu.Lock()
	defer verifierMu.Unlock()
	globalVerifiers[name] = fn
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func checkBindings(lic License, s *verifyState) cause {
	for _, b := range lic.Bindings {
		if !checkBinding(b, s) {
			switch {
			case b.Type == bindingMachine:
				return causeBindingMachineMismatch
			case b.Type == bindingPassword:
				return causeBindingPasswordMismatch
			case b.Type == bindingTOTP:
				return causeBindingTOTPMismatch
			default:
				return causeBindingCustomMismatch
			}
		}
	}
	return causeOK
}

func checkBinding(b Binding, s *verifyState) bool {
	switch {
	case b.Type == bindingMachine:
		if s.machineID == nil {
			return false
		}
		return contains(b.Value, string(s.machineID))
	case b.Type == bindingPassword:
		if s.password == "" {
			return false
		}
		p := argonParamsFor(b)
		got := argon2.IDKey([]byte(s.password), b.Salt, p.ArgonTime, p.ArgonMemory, p.ArgonThreads, p.ArgonKeyLen)
		return subtle.ConstantTimeCompare(got, b.Hash) == 1
	case b.Type == bindingTOTP:
		if s.totpCode == "" || len(b.Value) == 0 {
			return false
		}
		return totp.Verify(b.Value[0], s.totpCode, 1)
	case strings.HasPrefix(b.Type, bindingCustom):
		name := strings.TrimPrefix(b.Type, bindingCustom)
		verifierMu.RLock()
		fn, ok := globalVerifiers[name]
		verifierMu.RUnlock()
		if ok {
			return fn(b, s)
		}
		if val, ok := s.customVals[name]; ok {
			return contains(b.Value, val)
		}
		return false
	default:
		return false
	}
}
