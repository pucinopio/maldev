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
		// stop_servers_on_exit: when true, the manager batches a stop-all
		// command before tea.Quit so HTTP listeners drain cleanly without
		// the operator having to remember the [s] key per server. Distinct
		// from confirm_quit_with_servers which only adds a modal prompt.
		field.Bool("stop_servers_on_exit").Default(false),
		field.Enum("theme").Values("neon", "mono", "nord-soft").Default("neon"),
		// Apparence toggles. Wired into the runtime style helpers so the
		// operator's preferences persist across restarts. Pre-2026-05 these
		// were hardcoded checkboxes in the View() that did nothing.
		field.Bool("bold_saturated").Default(true),
		field.Bool("comfort_density").Default(false),
		field.Bool("timestamps_local").Default(false),
		field.Bytes("kek_salt"),
		field.Bytes("kek_canary"),
	}
}
