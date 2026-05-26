package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oioio-space/maldev/cleanup/memory"
	"github.com/oioio-space/maldev/internal/manager/crypto"
	"github.com/oioio-space/maldev/internal/manager/store"
	"github.com/oioio-space/maldev/internal/manager/store/ent"
	licenseent "github.com/oioio-space/maldev/internal/manager/store/ent/license"
	licensekg "github.com/oioio-space/maldev/license"
	"github.com/oioio-space/maldev/license/seal"
	"github.com/oioio-space/maldev/license/totp"
)

// LicenseService handles issuance, re-issuance, listing, and export of licences.
type LicenseService struct {
	store     *store.Store
	kek       *crypto.KEK
	audit     *AuditService
	issuer    *IssuerService
	identity  *IdentityService
	recipient *RecipientService
	totp      *TOTPService
}

func NewLicenseService(s *store.Store, k *crypto.KEK, a *AuditService,
	iss *IssuerService, id *IdentityService, rec *RecipientService, t *TOTPService) *LicenseService {
	return &LicenseService{
		store: s, kek: k, audit: a,
		issuer: iss, identity: id, recipient: rec, totp: t,
	}
}

// IssueRequest is the typed request that LicenseService.Issue accepts.
type IssueRequest struct {
	IssuerID     uuid.UUID
	Subject      string
	AudienceList []string
	NotBefore    time.Time
	NotAfter     time.Time
	Bindings     []BindingSpec
	Features     []string
	IdentityID   *uuid.UUID
	BinarySHA256 string
	Payload      json.RawMessage
	SealedFor    *uuid.UUID
	SealedPlain  []byte
	Label        string
	ReplacesID   *uuid.UUID
	Actor        string
}

// BindingSpec is the typed binding the wizard collects from the UI.
type BindingSpec struct {
	Type   string   // "machine" | "password" | "totp" | "custom:name"
	Values []string // machine ids OR custom values; password expects exactly 1 element (the password); totp expects 0
	Argon  *licensekg.BindingParams
	Label  string // for TOTP account label (defaults to Subject)
}

// IssuedLicense bundles the persisted row + the PEM artefact + any TOTP
// provisioning data the wizard needs to display.
type IssuedLicense struct {
	Row   *ent.License
	PEM   []byte
	TOTPs []TOTPProvisioning
}

// TOTPProvisioning carries the one-time artefacts shown to the operator after
// issuance. The secret is not stored in plaintext after this point.
type TOTPProvisioning struct {
	BindingIndex int
	Secret       string
	OtpauthURI   string
	QRImageASCII string
	QRImagePNG   []byte
}

// pendingTOTP collects TOTP state accumulated during binding construction so
// secrets can be persisted in the same transaction as the licence row.
type pendingTOTP struct {
	idx          int
	secret       string
	accountLabel string
}

// Issue signs a new licence, persists the row + any TOTP secrets, and
// returns the PEM bytes + provisioning artefacts.
func (svc *LicenseService) Issue(ctx context.Context, req IssueRequest) (*IssuedLicense, error) {
	if req.Subject == "" {
		return nil, errors.New("license: subject required")
	}
	issRow, err := svc.issuer.Get(ctx, req.IssuerID)
	if err != nil {
		return nil, fmt.Errorf("issuer lookup: %w", err)
	}
	priv, err := svc.issuer.PrivateKey(ctx, req.IssuerID)
	if err != nil {
		return nil, fmt.Errorf("issuer priv: %w", err)
	}
	defer memory.SecureZero(priv)

	// Build licencekg Bindings and collect TOTP secrets for later persistence.
	var bindings []licensekg.Binding
	var totps []pendingTOTP
	for i, b := range req.Bindings {
		switch {
		case b.Type == "machine":
			bindings = append(bindings, licensekg.BindMachineIDs(b.Values...))
		case b.Type == "password":
			if len(b.Values) != 1 {
				return nil, fmt.Errorf("binding %d: password expects exactly 1 value", i)
			}
			params := licensekg.DefaultArgon2idParams()
			if b.Argon != nil {
				params = *b.Argon
			}
			pwBinding, err := licensekg.BindPasswordWithParams(b.Values[0], params)
			if err != nil {
				return nil, fmt.Errorf("binding %d: %w", i, err)
			}
			bindings = append(bindings, pwBinding)
		case b.Type == "totp":
			secret, err := totp.NewSecret()
			if err != nil {
				return nil, fmt.Errorf("binding %d: totp secret: %w", i, err)
			}
			bindings = append(bindings, licensekg.BindTOTP(secret))
			label := b.Label
			if label == "" {
				label = req.Subject
			}
			totps = append(totps, pendingTOTP{idx: i, secret: secret, accountLabel: label})
		case strings.HasPrefix(b.Type, "custom:"):
			name := strings.TrimPrefix(b.Type, "custom:")
			bindings = append(bindings, licensekg.BindCustom(name, b.Values...))
		default:
			return nil, fmt.Errorf("binding %d: unknown type %q", i, b.Type)
		}
	}

	// Resolve optional identity SHA256.
	var identitySha string
	if req.IdentityID != nil {
		idRow, err := svc.identity.Get(ctx, *req.IdentityID)
		if err != nil {
			return nil, fmt.Errorf("identity lookup: %w", err)
		}
		identitySha = idRow.Sha256
	}

	// Encrypt sealed payload when a recipient is specified.
	var sealedPayload []byte
	payloadKind := licenseent.PayloadKindNone
	if req.SealedFor != nil {
		recRow, err := svc.recipient.Get(ctx, *req.SealedFor)
		if err != nil {
			return nil, fmt.Errorf("recipient lookup: %w", err)
		}
		s, err := seal.Seal(recRow.PublicKey, req.SealedPlain)
		if err != nil {
			return nil, fmt.Errorf("seal: %w", err)
		}
		sealedPayload = s
		payloadKind = licenseent.PayloadKindSealed
	} else if len(req.Payload) > 0 {
		payloadKind = licenseent.PayloadKindCleartext
	}

	opts := licensekg.IssueOptions{
		PrivateKey:     priv,
		KeyID:          issRow.KeyID,
		Issuer:         issRow.Name,
		Subject:        req.Subject,
		Audience:       req.AudienceList,
		NotBefore:      req.NotBefore,
		NotAfter:       req.NotAfter,
		Bindings:       bindings,
		Features:       req.Features,
		BinarySHA256:   req.BinarySHA256,
		IdentitySHA256: identitySha,
		Payload:        req.Payload,
		SealedPayload:  sealedPayload,
	}
	pemBytes, err := licensekg.Issue(opts)
	if err != nil {
		return nil, fmt.Errorf("issue: %w", err)
	}
	parsed, err := licensekg.Inspect(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("inspect after issue: %w", err)
	}

	// Persist row + TOTP secrets + audit in a single transaction.
	var licRow *ent.License
	var provs []TOTPProvisioning
	if err := withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		create := tx.License.Create().
			SetLicenseUUID(parsed.ID).
			SetSubject(parsed.Subject).
			SetIssuerName(parsed.Issuer).
			SetAudience(parsed.Audience).
			SetFeatures(parsed.Features).
			SetNotBefore(parsed.NotBefore).
			SetNotAfter(parsed.NotAfter).
			SetPayloadKind(payloadKind).
			SetPem(pemBytes).
			SetIssuerID(issRow.ID)
		if identitySha != "" {
			create = create.SetIdentitySha256(identitySha)
		}
		if req.BinarySHA256 != "" {
			create = create.SetBinarySha256(req.BinarySHA256)
		}
		if req.ReplacesID != nil {
			create = create.SetReplacesLicenseID(*req.ReplacesID)
		}
		var e error
		licRow, e = create.Save(ctx)
		if e != nil {
			return fmt.Errorf("persist license: %w", e)
		}

		// Persist TOTP secrets — one row per binding, KEK-wrapped.
		for _, p := range totps {
			wrapped, e := svc.kek.Wrap([]byte(p.secret))
			if e != nil {
				return e
			}
			if _, e = tx.TOTPSecret.Create().
				SetEncryptedSecret(wrapped).
				SetAccountLabel(p.accountLabel).
				SetLicenseID(licRow.ID).
				Save(ctx); e != nil {
				return e
			}
			ascii, _ := totp.QRImageASCIICompact(p.secret, p.accountLabel, issRow.Name)
			png, _ := totp.QRImagePNG(p.secret, p.accountLabel, issRow.Name, 256)
			provs = append(provs, TOTPProvisioning{
				BindingIndex: p.idx,
				Secret:       p.secret,
				OtpauthURI:   totp.URI(p.secret, p.accountLabel, issRow.Name),
				QRImageASCII: ascii,
				QRImagePNG:   png,
			})
		}

		return svc.audit.AppendTx(ctx, tx, "license.issue", req.Actor,
			Target{Kind: "License", ID: licRow.ID.String()},
			map[string]any{
				"subject":  req.Subject,
				"key_id":   issRow.KeyID,
				"audience": req.AudienceList,
				"features": req.Features,
				"bindings": len(req.Bindings),
			})
	}); err != nil {
		return nil, err
	}

	return &IssuedLicense{Row: licRow, PEM: pemBytes, TOTPs: provs}, nil
}

// ReIssueOptions configures ReIssue.
type ReIssueOptions struct {
	NotAfter time.Time
	Actor    string
}

// ReIssue creates a new licence from the original's data with overridable
// NotAfter. The new licence's ReplacesLicenseID points at the original,
// which is marked superseded.
//
// Password and TOTP bindings cannot be reconstructed — only machine and
// custom bindings are carried forward.
func (svc *LicenseService) ReIssue(ctx context.Context, originalID uuid.UUID, opts ReIssueOptions) (*IssuedLicense, error) {
	orig, err := svc.store.Client.License.Get(ctx, originalID)
	if err != nil {
		return nil, err
	}
	parsed, err := licensekg.Inspect(orig.Pem)
	if err != nil {
		return nil, fmt.Errorf("inspect original: %w", err)
	}

	var binds []BindingSpec
	for _, b := range parsed.Bindings {
		switch {
		case b.Type == "machine":
			binds = append(binds, BindingSpec{Type: "machine", Values: b.Value})
		case strings.HasPrefix(b.Type, "custom:"):
			binds = append(binds, BindingSpec{Type: b.Type, Values: b.Value})
		}
	}
	issuerRow, err := svc.store.Client.License.QueryIssuer(orig).Only(ctx)
	if err != nil {
		return nil, err
	}
	req := IssueRequest{
		IssuerID:     issuerRow.ID,
		Subject:      parsed.Subject,
		AudienceList: parsed.Audience,
		NotBefore:    parsed.NotBefore,
		NotAfter:     opts.NotAfter,
		Bindings:     binds,
		Features:     parsed.Features,
		BinarySHA256: parsed.BinarySHA256,
		Payload:      parsed.Payload,
		Label:        "re-issue of " + orig.Subject,
		ReplacesID:   &orig.ID,
		Actor:        opts.Actor,
	}
	if parsed.IdentitySHA256 != "" {
		idRows, _ := svc.identity.List(ctx)
		for _, ir := range idRows {
			if ir.Sha256 == parsed.IdentitySHA256 {
				req.IdentityID = &ir.ID
				break
			}
		}
	}

	issued, err := svc.Issue(ctx, req)
	if err != nil {
		return nil, err
	}
	// Supersede the original in a single tx so a crash between Issue and the
	// status update cannot leave the old licence active alongside the new one.
	if err := withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		if _, err := tx.License.UpdateOneID(originalID).
			SetStatus(licenseent.StatusSuperseded).
			Save(ctx); err != nil {
			return err
		}
		return svc.audit.AppendTx(ctx, tx, "license.reissue", opts.Actor,
			Target{Kind: "License", ID: issued.Row.ID.String()},
			map[string]any{"replaces": originalID.String()})
	}); err != nil {
		// New licence is already signed and persisted; surface the inconsistency
		// so the caller can retry or alert, but don't discard the issued licence.
		return issued, fmt.Errorf("supersede original: %w", err)
	}
	return issued, nil
}

// ListFilter is the typed filter the UI passes to List.
type ListFilter struct {
	Status         string
	SubjectContain string
	KeyID          string
	Feature        string
	ExpireBefore   *time.Time
	Limit          int
}

func (svc *LicenseService) List(ctx context.Context, f ListFilter) ([]*ent.License, error) {
	q := svc.store.Client.License.Query()
	if f.Status != "" {
		q = q.Where(licenseent.StatusEQ(licenseent.Status(f.Status)))
	}
	if f.SubjectContain != "" {
		q = q.Where(licenseent.SubjectContains(f.SubjectContain))
	}
	if f.ExpireBefore != nil {
		q = q.Where(licenseent.NotAfterLT(*f.ExpireBefore))
	}
	if f.Limit <= 0 {
		f.Limit = 500
	}
	return q.Order(ent.Desc(licenseent.FieldCreatedAt)).Limit(f.Limit).All(ctx)
}

func (svc *LicenseService) Get(ctx context.Context, id uuid.UUID) (*ent.License, error) {
	return svc.store.Client.License.Get(ctx, id)
}

// LicenseChain holds the parent-walk and successor-walk for a single licence.
type LicenseChain struct {
	// Parents is ordered oldest-first (root at index 0).
	Parents []*ent.License
	// This is the licence the chain was built around.
	This *ent.License
	// Successors is ordered newest-last (direct successor at index 0).
	Successors []*ent.License
}

// GetChain resolves the full parent/successor lineage for id.
// It walks ReplacesLicenseID backwards (up to 20 hops) for parents, and
// queries all rows whose ReplacesLicenseID equals each node for successors.
func (svc *LicenseService) GetChain(ctx context.Context, id uuid.UUID) (*LicenseChain, error) {
	root, err := svc.store.Client.License.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get chain root: %w", err)
	}
	chain := &LicenseChain{This: root}

	// Walk backwards: follow ReplacesLicenseID until nil or > 20 hops.
	cur := root
	for range 20 {
		if cur.ReplacesLicenseID == nil {
			break
		}
		parent, err := svc.store.Client.License.Get(ctx, *cur.ReplacesLicenseID)
		if err != nil {
			break // parent may have been deleted; stop gracefully
		}
		chain.Parents = append(chain.Parents, parent)
		cur = parent
	}
	// Reverse so oldest is first.
	for i, j := 0, len(chain.Parents)-1; i < j; i, j = i+1, j-1 {
		chain.Parents[i], chain.Parents[j] = chain.Parents[j], chain.Parents[i]
	}

	// Walk forward: direct successors are rows where ReplacesLicenseID == id.
	succs, err := svc.store.Client.License.Query().
		Where(licenseent.ReplacesLicenseIDEQ(id)).
		Order(ent.Asc(licenseent.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list successors: %w", err)
	}
	chain.Successors = succs
	return chain, nil
}

func (svc *LicenseService) GetByUUID(ctx context.Context, licUUID string) (*ent.License, error) {
	return svc.store.Client.License.Query().Where(licenseent.LicenseUUIDEQ(licUUID)).Only(ctx)
}

// Inspect is a thin wrapper over licensekg.Inspect for use by the TUI when
// displaying any PEM (not necessarily one already in the DB).
func (svc *LicenseService) Inspect(pem []byte) (*licensekg.License, error) {
	return licensekg.Inspect(pem)
}

// Import parses a PEM and persists it. The signature is NOT verified here —
// the operator imports licences they trust (from a backup or another instance).
func (svc *LicenseService) Import(ctx context.Context, pemBytes []byte, label, actor string) (*ent.License, error) {
	parsed, err := licensekg.Inspect(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("inspect: %w", err)
	}
	issuers, err := svc.issuer.List(ctx)
	if err != nil {
		return nil, err
	}
	var issuerID *uuid.UUID
	for _, i := range issuers {
		if i.KeyID == parsed.KeyID {
			id := i.ID
			issuerID = &id
			break
		}
	}
	if issuerID == nil {
		return nil, fmt.Errorf("no issuer registered for KeyID %q", parsed.KeyID)
	}

	payloadKind := licenseent.PayloadKindNone
	if len(parsed.SealedPayload) > 0 {
		payloadKind = licenseent.PayloadKindSealed
	} else if len(parsed.Payload) > 0 {
		payloadKind = licenseent.PayloadKindCleartext
	}
	var row *ent.License
	if err := withTx(ctx, svc.store, func(ctx context.Context, tx *ent.Tx) error {
		create := tx.License.Create().
			SetLicenseUUID(parsed.ID).
			SetSubject(parsed.Subject).
			SetIssuerName(parsed.Issuer).
			SetAudience(parsed.Audience).
			SetFeatures(parsed.Features).
			SetNotBefore(parsed.NotBefore).
			SetNotAfter(parsed.NotAfter).
			SetPayloadKind(payloadKind).
			SetPem(pemBytes).
			SetIssuerID(*issuerID)
		if parsed.IdentitySHA256 != "" {
			create = create.SetIdentitySha256(parsed.IdentitySHA256)
		}
		if parsed.BinarySHA256 != "" {
			create = create.SetBinarySha256(parsed.BinarySHA256)
		}
		var e error
		row, e = create.Save(ctx)
		if e != nil {
			return e
		}
		return svc.audit.AppendTx(ctx, tx, "license.import", actor,
			Target{Kind: "License", ID: row.ID.String()},
			map[string]any{"label": label})
	}); err != nil {
		return nil, err
	}
	return row, nil
}

// ExportPEM returns a copy of the canonical PEM bytes stored for this licence.
func (svc *LicenseService) ExportPEM(ctx context.Context, id uuid.UUID) ([]byte, error) {
	row, err := svc.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(row.Pem))
	copy(out, row.Pem)
	return out, nil
}

// ExportBatch produces a tar.gz archive of multiple licence PEMs, each named
// by subject and license UUID.
func (svc *LicenseService) ExportBatch(ctx context.Context, ids []uuid.UUID) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, id := range ids {
		row, err := svc.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		name := fmt.Sprintf("%s-%s.license", row.Subject, row.LicenseUUID)
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(row.Pem)),
		}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(row.Pem); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// HashFile computes the SHA-256 digest of a file at path, reporting progress
// via the optional callback. Returns the lowercase hex digest.
func (svc *LicenseService) HashFile(ctx context.Context, path string, progress func(read, total int64)) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	total := info.Size()
	h := sha256.New()
	buf := make([]byte, 64*1024)
	var read int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			read += int64(n)
			if progress != nil {
				progress(read, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
