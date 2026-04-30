package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ProviderAccount struct {
	ent.Schema
}

func (ProviderAccount) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (ProviderAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "provider_account"},
	}
}

func (ProviderAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.String("account_uuid").
			Unique(),
		field.String("access_token"),
		field.String("refresh_token"),
		field.String("credentials"),
	}
}

func (ProviderAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("fake_tokens", FakeToken.Type),
		edge.To("granted_users", UserGrantedProviderAccount.Type),
	}
}

func (ProviderAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("access_token"),
		index.Fields("refresh_token"),
	}
}
