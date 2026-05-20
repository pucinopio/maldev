package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type TOTPSecret struct{ ent.Schema }

func (TOTPSecret) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.Bytes("encrypted_secret"),
		field.String("account_label"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (TOTPSecret) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("license", License.Type).Ref("totps").Unique().Required(),
	}
}
