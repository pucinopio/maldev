package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type ServerConfig struct{ ent.Schema }

func (ServerConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Unique().Immutable(),
		field.String("revocation_listen").Default(":8443"),
		field.String("revocation_tls_cert").Optional(),
		field.String("revocation_tls_key").Optional(),
		field.Bytes("revocation_admin_token_enc").Optional(),
		field.String("revocation_path").Default("/revoked.pem"),
		field.String("heartbeat_listen").Default(":8444"),
		field.String("heartbeat_tls_cert").Optional(),
		field.String("heartbeat_tls_key").Optional(),
		field.String("heartbeat_path").Default("/heartbeat"),
		field.String("probe_listen").Default(":8445"),
		field.String("probe_tls_cert").Optional(),
		field.String("probe_tls_key").Optional(),
		field.Int("probe_default_ttl_seconds").Default(86400),
	}
}
