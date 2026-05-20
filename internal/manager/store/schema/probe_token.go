package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ProbeToken struct{ ent.Schema }

func (ProbeToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique().NotEmpty(),
		field.String("label").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("expires_at"),
		field.Time("used_at").Optional().Nillable(),
		field.String("remote_addr").Optional(),
		field.String("hostname").Optional(),
		field.String("os").Optional(),
		field.String("arch").Optional(),
		field.String("cpu_brand").Optional(),
		field.String("local_hex").Optional(),
		field.String("composite_hex").Optional(),
	}
}

func (ProbeToken) Indexes() []ent.Index {
	return []ent.Index{index.Fields("expires_at")}
}
