package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

type License struct{ ent.Schema }

func (License) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("license_uuid").Unique().NotEmpty(),
		field.String("subject").NotEmpty(),
		field.String("issuer_name").Optional(),
		field.JSON("audience", []string{}).Optional(),
		field.JSON("features", []string{}).Optional(),
		field.Time("not_before"),
		field.Time("not_after"),
		field.JSON("bindings_meta", map[string]any{}).Optional(),
		field.Enum("payload_kind").Values("none", "cleartext", "sealed").Default("none"),
		field.String("identity_sha256").Optional(),
		field.String("binary_sha256").Optional(),
		field.Bytes("pem"),
		field.Enum("status").Values("active", "revoked", "expired", "superseded").Default("active"),
		field.UUID("replaces_license_id", uuid.UUID{}).Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (License) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("issuer", Issuer.Type).Ref("licenses").Unique().Required(),
		edge.To("totps", TOTPSecret.Type),
		edge.To("revocation", Revocation.Type).Unique(),
	}
}

func (License) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("subject"),
		index.Fields("status"),
		index.Fields("not_after"),
		index.Fields("identity_sha256"),
	}
}
