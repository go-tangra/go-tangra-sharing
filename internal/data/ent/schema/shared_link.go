package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// SharedLink holds the schema definition for the SharedLink entity.
// SharedLinks store one-time share tokens for secrets and documents.
type SharedLink struct {
	ent.Schema
}

// Annotations of the SharedLink.
func (SharedLink) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sharing_shared_links"},
		entsql.WithComments(true),
	}
}

// Fields of the SharedLink.
func (SharedLink) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.Enum("resource_type").
			Values("SECRET", "DOCUMENT").
			Comment("Type of resource being shared"),

		field.String("resource_id").
			NotEmpty().
			MaxLen(255).
			Comment("ID of the shared resource"),

		field.String("resource_name").
			NotEmpty().
			MaxLen(255).
			Comment("Display name of the shared resource"),

		field.String("token").
			NotEmpty().
			MaxLen(64).
			Unique().
			Comment("Unique share token (64 hex chars)"),

		field.Bytes("encrypted_content").
			Comment("AES-256-GCM encrypted content"),

		field.Bytes("encryption_nonce").
			Comment("AES-256-GCM nonce"),

		field.String("recipient_email").
			NotEmpty().
			MaxLen(320).
			Comment("Recipient email address"),

		field.String("message").
			Optional().
			MaxLen(2048).
			Comment("Optional message to recipient"),

		field.String("template_id").
			Optional().
			Nillable().
			MaxLen(36).
			Comment("Email template ID used"),

		field.Bool("viewed").
			Default(false).
			Comment("Whether the share has been viewed"),

		field.Time("viewed_at").
			Optional().
			Nillable().
			Comment("When the share was viewed"),

		field.String("viewed_ip").
			Optional().
			MaxLen(45).
			Comment("IP address of viewer"),

		field.Bool("revoked").
			Default(false).
			Comment("Whether the share has been revoked"),
	}
}

// Edges of the SharedLink.
func (SharedLink) Edges() []ent.Edge {
	return nil
}

// Mixin of the SharedLink.
func (SharedLink) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the SharedLink.
func (SharedLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token").Unique(),
		index.Fields("resource_type", "resource_id"),
		index.Fields("tenant_id"),
		index.Fields("recipient_email"),
		index.Fields("tenant_id", "viewed"),
	}
}
