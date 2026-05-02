package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AnthropicUsage struct {
	ent.Schema
}

func (AnthropicUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "anthropic_usage"},
	}
}

func (AnthropicUsage) Fields() []ent.Field {
	return []ent.Field{
		field.String("device_id"),
		field.String("session_id"),
		field.String("account_uuid"),
		field.String("model").
			Default(""),
		field.Int64("input_tokens"),
		field.Int64("cache_read_input_tokens"),
		field.Int64("cache_creation_input_tokens"),
		field.Int64("output_tokens"),
		field.Int64("cache_creation_ephemeral_5m_input_tokens"),
		field.Int64("cache_creation_ephemeral_1h_input_tokens"),
		field.Float("cache_hit_rate"),
		field.Text("raw"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now),
	}
}

func (AnthropicUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
		index.Fields("cache_hit_rate"),
	}
}
