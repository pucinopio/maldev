package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type Revocation struct{ ent.Schema }

func (Revocation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("reason"),
		field.Time("revoked_at").Default(time.Now).Immutable(),
		field.String("revoked_by"),
	}
}

func (Revocation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("license", License.Type).Ref("revocation").Unique().Required(),
	}
}
