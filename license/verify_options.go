package license

import (
	"context"
	"log/slog"
	"time"

	"github.com/oioio-space/maldev/license/heartbeat"
	"github.com/oioio-space/maldev/license/revoke"
)

// VerifyOption configures Verify. See WithAudience, WithIssuer, etc.
type VerifyOption func(*verifyState)

type verifyState struct {
	ctx          context.Context
	clock        Clock
	logger       *slog.Logger
	maxClockSkew time.Duration

	audience []string
	issuer   string

	machineID  []byte
	password   string
	customVals map[string]string

	statePath     string
	stateHostIDFn func() ([]byte, error)

	binaryPinning bool
	identityBytes []byte

	revokeSource    revoke.RevocationSource
	revokeRefresh   time.Duration
	revokeCachePath string
	gracePeriod     time.Duration

	heartbeatClient   heartbeat.Client
	heartbeatInterval time.Duration

	warnings []string
}

func newVerifyState(opts []VerifyOption) *verifyState {
	s := &verifyState{
		ctx:          context.Background(),
		clock:        realClock{},
		logger:       slog.Default(),
		maxClockSkew: 5 * time.Minute,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func WithContext(ctx context.Context) VerifyOption {
	return func(s *verifyState) { s.ctx = ctx }
}

func WithClock(c Clock) VerifyOption {
	return func(s *verifyState) {
		if c != nil {
			s.clock = c
		}
	}
}

func WithLogger(l *slog.Logger) VerifyOption {
	return func(s *verifyState) {
		if l != nil {
			s.logger = l
		}
	}
}

func WithMaxClockSkew(d time.Duration) VerifyOption {
	return func(s *verifyState) { s.maxClockSkew = d }
}

func WithAudience(aud ...string) VerifyOption {
	return func(s *verifyState) { s.audience = append(s.audience, aud...) }
}

func WithIssuer(iss string) VerifyOption {
	return func(s *verifyState) { s.issuer = iss }
}

func WithMachineID(id []byte) VerifyOption {
	return func(s *verifyState) { s.machineID = append([]byte(nil), id...) }
}

func WithPassword(p string) VerifyOption {
	return func(s *verifyState) { s.password = p }
}

func WithCustom(name, value string) VerifyOption {
	return func(s *verifyState) {
		if s.customVals == nil {
			s.customVals = map[string]string{}
		}
		s.customVals[name] = value
	}
}

func WithStateFile(path string) VerifyOption {
	return func(s *verifyState) { s.statePath = path }
}

func WithStateHostID(fn func() ([]byte, error)) VerifyOption {
	return func(s *verifyState) { s.stateHostIDFn = fn }
}

func WithBinaryPinning() VerifyOption {
	return func(s *verifyState) { s.binaryPinning = true }
}

func WithIdentityBytes(b []byte) VerifyOption {
	return func(s *verifyState) { s.identityBytes = append([]byte(nil), b...) }
}

func WithRevocation(src revoke.RevocationSource, refresh time.Duration, cachePath string) VerifyOption {
	return func(s *verifyState) {
		s.revokeSource = src
		s.revokeRefresh = refresh
		s.revokeCachePath = cachePath
	}
}

func WithGracePeriod(d time.Duration) VerifyOption {
	return func(s *verifyState) { s.gracePeriod = d }
}

func WithHeartbeat(c heartbeat.Client, interval time.Duration) VerifyOption {
	return func(s *verifyState) {
		s.heartbeatClient = c
		s.heartbeatInterval = interval
	}
}
