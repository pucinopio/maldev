package license

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	bindingMachine  = "machine"
	bindingPassword = "password"
	bindingCustom   = "custom:" // prefix
)

// Argon2id parameters chosen for ~100 ms on a 2024-era laptop CPU. Stored
// next to salt/hash in the binding so future tuning is forward compatible.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 16
)

// BindMachineIDs builds a binding accepting any of the listed machine ids.
func BindMachineIDs(ids ...string) Binding {
	return Binding{Type: bindingMachine, Value: append([]string(nil), ids...)}
}

// BindCustom builds a typed custom binding. Multiple values accept any-match.
func BindCustom(name string, values ...string) Binding {
	return Binding{Type: bindingCustom + name, Value: append([]string(nil), values...)}
}

// BindPassword derives argon2id(salt, password). The plaintext is never
// retained.
func BindPassword(password string) (Binding, error) {
	if password == "" {
		return Binding{}, errors.New("license: empty password")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return Binding{}, err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return Binding{Type: bindingPassword, Hash: hash, Salt: salt}, nil
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
		got := argon2.IDKey([]byte(s.password), b.Salt, argonTime, argonMemory, argonThreads, argonKeyLen)
		return subtle.ConstantTimeCompare(got, b.Hash) == 1
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
