package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/tx7do/go-crud/entgo/mixin"
)

// EmailTemplate holds the schema definition for the EmailTemplate entity.
// EmailTemplates store customizable email templates for sharing notifications.
type EmailTemplate struct {
	ent.Schema
}

// Annotations of the EmailTemplate.
func (EmailTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "sharing_email_templates"},
		entsql.WithComments(true),
	}
}

// Fields of the EmailTemplate.
func (EmailTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("UUID primary key"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Template name"),

		field.String("subject").
			NotEmpty().
			MaxLen(1024).
			Comment("Email subject (Go template)"),

		field.Text("html_body").
			NotEmpty().
			Comment("Email HTML body (Go html/template)"),

		field.Bool("is_default").
			Default(false).
			Comment("Whether this is the default template for the tenant"),
	}
}

// Edges of the EmailTemplate.
func (EmailTemplate) Edges() []ent.Edge {
	return nil
}

// Mixin of the EmailTemplate.
func (EmailTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

// Indexes of the EmailTemplate.
func (EmailTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
		index.Fields("tenant_id", "is_default"),
		index.Fields("tenant_id"),
	}
}
