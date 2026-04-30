package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type UserGrantedProviderAccount struct {
	ent.Schema
}

func (UserGrantedProviderAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_granted_provider_account"},
	}
}

func (UserGrantedProviderAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("user_id"),
		field.Int64("provider_account_id"),
	}
}

func (UserGrantedProviderAccount) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("granted_provider_accounts").
			Field("user_id").
			Required().
			Unique(),
		edge.From("provider_account", ProviderAccount.Type).
			Ref("granted_users").
			Field("provider_account_id").
			Required().
			Unique(),
	}
}
