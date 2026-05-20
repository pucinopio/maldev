package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Setting struct{ ent.Schema }

func (Setting) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Unique().Immutable(),
		field.String("default_issuer_name").Optional(),
		field.JSON("default_audience", []string{}).Optional(),
		field.Int("default_ttl_seconds").Default(30 * 86400),
		field.Enum("default_argon_preset").Values("fast", "default", "paranoid").Default("default"),
		field.String("operator_name").Optional(),
		field.Bool("auto_start_servers").Default(false),
		field.Bool("confirm_quit_with_servers").Default(true),
		field.Bytes("kek_salt"),
		field.Bytes("kek_canary"),
	}
}
