package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// SharePolicy holds the schema definition for the SharePolicy entity.
// SharePolicies store access restriction rules for shared links.
type SharePolicy struct {
	ent.Schema
}

// Annotations of the SharePolicy.
func (SharePolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sharing_share_policies"},
		entsql.WithComments(true),
	}
}

// Fields of the SharePolicy.
func (SharePolicy) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("share_link_id").
			NotEmpty().
			MaxLen(36).
			Comment("FK to shared_link.id"),

		field.Enum("type").
			Values("BLACKLIST", "WHITELIST").
			Comment("Restriction type: BLACKLIST (deny) or WHITELIST (allow)"),

		field.Enum("method").
			Values("IP", "MAC", "REGION", "TIME", "DEVICE", "NETWORK").
			Comment("Restriction method"),

		field.String("value").
			NotEmpty().
			MaxLen(512).
			Comment("Restriction value (IP, CIDR range, MAC, region code, time range, device ID)"),

		field.String("reason").
			Optional().
			MaxLen(1024).
			Comment("Explanation for this restriction"),
	}
}

// Edges of the SharePolicy.
func (SharePolicy) Edges() []ent.Edge {
	return nil
}

// Mixin of the SharePolicy.
func (SharePolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the SharePolicy.
func (SharePolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("share_link_id"),
		index.Fields("share_link_id", "type", "method"),
	}
}
