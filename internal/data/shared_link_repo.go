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
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharedlink"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// SharedLinkRepo handles database operations for shared links
type SharedLinkRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewSharedLinkRepo creates a new SharedLinkRepo
func NewSharedLinkRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *SharedLinkRepo {
	return &SharedLinkRepo{
		log:       ctx.NewLoggerHelper("sharing/repo/shared_link"),
		entClient: entClient,
	}
}

// Create creates a new shared link
func (r *SharedLinkRepo) Create(ctx context.Context, tenantID uint32, resourceType, resourceID, resourceName, token string, encryptedContent, nonce []byte, recipientEmail, message, templateID string, createdBy *uint32) (*ent.SharedLink, error) {
	id := uuid.New().String()

	builder := r.entClient.Client().SharedLink.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetResourceType(sharedlink.ResourceType(resourceType)).
		SetResourceID(resourceID).
		SetResourceName(resourceName).
		SetToken(token).
		SetEncryptedContent(encryptedContent).
		SetEncryptionNonce(nonce).
		SetRecipientEmail(recipientEmail).
		SetViewed(false).
		SetRevoked(false).
		SetCreateTime(time.Now())

	if message != "" {
		builder.SetMessage(message)
	}
	if templateID != "" {
		builder.SetTemplateID(templateID)
	}
	if createdBy != nil {
		builder.SetCreateBy(*createdBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create shared link failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("create shared link failed")
	}

	return entity, nil
}

// GetByToken retrieves a shared link by token
func (r *SharedLinkRepo) GetByToken(ctx context.Context, token string) (*ent.SharedLink, error) {
	entity, err := r.entClient.Client().SharedLink.Query().
		Where(sharedlink.TokenEQ(token)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get shared link by token failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("get shared link failed")
	}
	return entity, nil
}

// GetByID retrieves a shared link by ID
func (r *SharedLinkRepo) GetByID(ctx context.Context, id string) (*ent.SharedLink, error) {
	entity, err := r.entClient.Client().SharedLink.Query().
		Where(sharedlink.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf("get shared link failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("get shared link failed")
	}
	return entity, nil
}

// ListByTenant lists shared links for a tenant with pagination
func (r *SharedLinkRepo) ListByTenant(ctx context.Context, tenantID uint32, resourceType *string, recipientEmail *string, page, pageSize uint32) ([]*ent.SharedLink, int, error) {
	query := r.entClient.Client().SharedLink.Query().
		Where(sharedlink.TenantIDEQ(tenantID))

	if resourceType != nil && *resourceType != "" {
		query = query.Where(sharedlink.ResourceTypeEQ(sharedlink.ResourceType(*resourceType)))
	}

	if recipientEmail != nil && *recipientEmail != "" {
		query = query.Where(sharedlink.RecipientEmailEQ(*recipientEmail))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		r.log.Errorf("count shared links failed: %s", err.Error())
		return nil, 0, sharingV1.ErrorInternalServerError("count shared links failed")
	}

	if page > 0 && pageSize > 0 {
		offset := int((page - 1) * pageSize)
		query = query.Offset(offset).Limit(int(pageSize))
	}

	entities, err := query.
		Order(ent.Desc(sharedlink.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list shared links failed: %s", err.Error())
		return nil, 0, sharingV1.ErrorInternalServerError("list shared links failed")
	}

	return entities, total, nil
}

// MarkViewed marks a shared link as viewed
func (r *SharedLinkRepo) MarkViewed(ctx context.Context, id string, viewerIP string) error {
	now := time.Now()
	builder := r.entClient.Client().SharedLink.UpdateOneID(id).
		SetViewed(true).
		SetViewedAt(now)

	if viewerIP != "" {
		builder.SetViewedIP(viewerIP)
	}

	_, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("mark shared link viewed failed: %s", err.Error())
		return sharingV1.ErrorInternalServerError("mark shared link viewed failed")
	}
	return nil
}

// Revoke revokes a shared link
func (r *SharedLinkRepo) Revoke(ctx context.Context, id string) error {
	_, err := r.entClient.Client().SharedLink.UpdateOneID(id).
		SetRevoked(true).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return sharingV1.ErrorShareNotFound("share not found")
		}
		r.log.Errorf("revoke shared link failed: %s", err.Error())
		return sharingV1.ErrorInternalServerError("revoke shared link failed")
	}
	return nil
}

// ToProto converts an ent.SharedLink to sharingV1.SharedLink
func (r *SharedLinkRepo) ToProto(entity *ent.SharedLink) *sharingV1.SharedLink {
	if entity == nil {
		return nil
	}

	proto := &sharingV1.SharedLink{
		Id:             entity.ID,
		TenantId:       derefUint32(entity.TenantID),
		ResourceId:     entity.ResourceID,
		ResourceName:   entity.ResourceName,
		Token:          entity.Token,
		RecipientEmail: entity.RecipientEmail,
		Message:        entity.Message,
		Viewed:         entity.Viewed,
		Revoked:        entity.Revoked,
	}

	switch entity.ResourceType {
	case sharedlink.ResourceTypeSECRET:
		proto.ResourceType = sharingV1.ResourceType_RESOURCE_TYPE_SECRET
	case sharedlink.ResourceTypeDOCUMENT:
		proto.ResourceType = sharingV1.ResourceType_RESOURCE_TYPE_DOCUMENT
	}

	if entity.CreateBy != nil {
		proto.CreatedBy = entity.CreateBy
	}

	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreateTime = timestamppb.New(*entity.CreateTime)
	}

	if entity.ViewedAt != nil && !entity.ViewedAt.IsZero() {
		proto.ViewedAt = timestamppb.New(*entity.ViewedAt)
	}

	return proto
}

// derefUint32 safely dereferences a *uint32 pointer
func derefUint32(v *uint32) uint32 {
	if v == nil {
		return 0
	}
	return *v
}
