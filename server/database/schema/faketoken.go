package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type FakeToken struct {
	ent.Schema
}

func (FakeToken) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "fake_token"},
	}
}

func (FakeToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("user_id"),
		field.Int64("provider_account_id"),
		field.String("access_token"),
		field.String("refresh_token"),
	}
}

func (FakeToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("fake_tokens").
			Field("user_id").
			Required().
			Unique(),
		edge.From("provider_account", ProviderAccount.Type).
			Ref("fake_tokens").
			Field("provider_account_id").
			Required().
			Unique(),
	}
}

func (FakeToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("access_token"),
		index.Fields("refresh_token"),
		index.Fields("user_id", "provider_account_id").
			Unique(),
	}
}
