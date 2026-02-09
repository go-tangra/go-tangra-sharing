package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-sharing/internal/data/ent"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/emailtemplate"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// EmailTemplateRepo handles database operations for email templates
type EmailTemplateRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewEmailTemplateRepo creates a new EmailTemplateRepo
func NewEmailTemplateRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *EmailTemplateRepo {
	return &EmailTemplateRepo{
		log:       ctx.NewLoggerHelper("sharing/repo/email_template"),
		entClient: entClient,
	}
}

// Create creates a new email template
func (r *EmailTemplateRepo) Create(ctx context.Context, tenantID uint32, name, subject, htmlBody string, isDefault bool, createdBy *uint32) (*ent.EmailTemplate, error) {
	id := uuid.New().String()

	// If this is being set as default, unset other defaults first
	if isDefault {
		r.unsetDefaults(ctx, tenantID)
	}

	builder := r.entClient.Client().EmailTemplate.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetName(name).
		SetSubject(subject).
		SetHTMLBody(htmlBody).
		SetIsDefault(isDefault).
		SetCreateTime(time.Now())

	if createdBy != nil {
		builder.SetCreateBy(*createdBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, sharingV1.ErrorTemplateAlreadyExists("template with this name already exists")
		}
		r.log.Errorf("create email template failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("create email template failed")
	}

	return entity, nil
}

// GetByID retrieves an email template by ID
func (r *EmailTemplateRepo) GetByID(ctx context.Context, id string) (*ent.EmailTemplate, error) {
	entity, err := r.entClient.Client().EmailTemplate.Query().
		Where(emailtemplate.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get email template failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("get email template failed")
	}
	return entity, nil
}

// GetDefault retrieves the default email template for a tenant
func (r *EmailTemplateRepo) GetDefault(ctx context.Context, tenantID uint32) (*ent.EmailTemplate, error) {
	entity, err := r.entClient.Client().EmailTemplate.Query().
		Where(
			emailtemplate.TenantIDEQ(tenantID),
			emailtemplate.IsDefaultEQ(true),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get default email template failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("get default email template failed")
	}
	return entity, nil
}

// ListByTenant lists email templates for a tenant
func (r *EmailTemplateRepo) ListByTenant(ctx context.Context, tenantID uint32, page, pageSize uint32) ([]*ent.EmailTemplate, int, error) {
	query := r.entClient.Client().EmailTemplate.Query().
		Where(emailtemplate.TenantIDEQ(tenantID))

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count email templates failed: %s", err.Error())
		return nil, 0, sharingV1.ErrorInternalServerError("count email templates failed")
	}

	if page > 0 && pageSize > 0 {
		offset := int((page - 1) * pageSize)
		query = query.Offset(offset).Limit(int(pageSize))
	}

	entities, err := query.
		Order(ent.Desc(emailtemplate.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list email templates failed: %s", err.Error())
		return nil, 0, sharingV1.ErrorInternalServerError("list email templates failed")
	}

	return entities, total, nil
}

// Update updates an email template
func (r *EmailTemplateRepo) Update(ctx context.Context, id string, tenantID uint32, name, subject, htmlBody *string, isDefault *bool, updatedBy *uint32) (*ent.EmailTemplate, error) {
	builder := r.entClient.Client().EmailTemplate.UpdateOneID(id).
		SetUpdateTime(time.Now())

	if name != nil {
		builder.SetName(*name)
	}
	if subject != nil {
		builder.SetSubject(*subject)
	}
	if htmlBody != nil {
		builder.SetHTMLBody(*htmlBody)
	}
	if isDefault != nil {
		if *isDefault {
			r.unsetDefaults(ctx, tenantID)
		}
		builder.SetIsDefault(*isDefault)
	}
	if updatedBy != nil {
		builder.SetUpdateBy(*updatedBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, sharingV1.ErrorTemplateNotFound("template not found")
		}
		if ent.IsConstraintError(err) {
			return nil, sharingV1.ErrorTemplateAlreadyExists("template with this name already exists")
		}
		r.log.Errorf("update email template failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("update email template failed")
	}

	return entity, nil
}

// Delete deletes an email template
func (r *EmailTemplateRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().EmailTemplate.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return sharingV1.ErrorTemplateNotFound("template not found")
		}
		r.log.Errorf("delete email template failed: %s", err.Error())
		return sharingV1.ErrorInternalServerError("delete email template failed")
	}
	return nil
}

// ToProto converts an ent.EmailTemplate to sharingV1.EmailTemplate
func (r *EmailTemplateRepo) ToProto(entity *ent.EmailTemplate) *sharingV1.EmailTemplate {
	if entity == nil {
		return nil
	}

	proto := &sharingV1.EmailTemplate{
		Id:        entity.ID,
		TenantId:  derefUint32(entity.TenantID),
		Name:      entity.Name,
		Subject:   entity.Subject,
		HtmlBody:  entity.HTMLBody,
		IsDefault: entity.IsDefault,
	}

	if entity.CreateBy != nil {
		proto.CreatedBy = entity.CreateBy
	}
	if entity.UpdateBy != nil {
		proto.UpdatedBy = entity.UpdateBy
	}
	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreateTime = timestamppb.New(*entity.CreateTime)
	}
	if entity.UpdateTime != nil && !entity.UpdateTime.IsZero() {
		proto.UpdateTime = timestamppb.New(*entity.UpdateTime)
	}

	return proto
}

// unsetDefaults clears the is_default flag on all templates for a tenant
func (r *EmailTemplateRepo) unsetDefaults(ctx context.Context, tenantID uint32) {
	_, err := r.entClient.Client().EmailTemplate.Update().
		Where(
			emailtemplate.TenantIDEQ(tenantID),
			emailtemplate.IsDefaultEQ(true),
		).
		SetIsDefault(false).
		Save(ctx)
	if err != nil {
		r.log.Warnf("failed to unset default templates: %v", err)
	}
}
