package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"

	"github.com/google/uuid"
)

type RecipientKey struct{ ent.Schema }

func (RecipientKey) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name").Unique().NotEmpty(),
		field.Bytes("public_key").MaxLen(32).MinLen(32),
		field.Bytes("encrypted_priv"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}
