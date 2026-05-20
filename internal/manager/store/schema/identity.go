package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid"
)

type Identity struct{ ent.Schema }

func (Identity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name").Unique().NotEmpty(),
		field.Bytes("bytes").MaxLen(32).MinLen(32),
		field.String("sha256").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Identity) Indexes() []ent.Index {
	return []ent.Index{index.Fields("sha256")}
}
