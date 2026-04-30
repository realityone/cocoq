package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user"},
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.String("oauth_state_token"),
		field.Int64("state"),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("fake_tokens", FakeToken.Type),
		edge.To("granted_provider_accounts", UserGrantedProviderAccount.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("oauth_state_token"),
	}
}
