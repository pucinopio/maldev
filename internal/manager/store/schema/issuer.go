package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type Issuer struct{ ent.Schema }

func (Issuer) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name").NotEmpty(),
		field.String("key_id").Unique().NotEmpty(),
		field.Bytes("public_key").MaxLen(32).MinLen(32),
		field.Bytes("encrypted_priv"),
		field.Bool("active").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("retired_at").Optional().Nillable(),
	}
}

func (Issuer) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("licenses", License.Type),
	}
}
