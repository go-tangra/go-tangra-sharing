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
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharepolicy"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// SharePolicyRepo handles database operations for share policies
type SharePolicyRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewSharePolicyRepo creates a new SharePolicyRepo
func NewSharePolicyRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *SharePolicyRepo {
	return &SharePolicyRepo{
		log:       ctx.NewLoggerHelper("sharing/repo/share_policy"),
		entClient: entClient,
	}
}

// Create creates a new share policy
func (r *SharePolicyRepo) Create(ctx context.Context, tenantID uint32, shareLinkID, policyType, method, value, reason string, createdBy *uint32) (*ent.SharePolicy, error) {
	id := uuid.New().String()

	builder := r.entClient.Client().SharePolicy.Create().
		SetID(id).
		SetTenantID(tenantID).
		SetShareLinkID(shareLinkID).
		SetType(sharepolicy.Type(policyType)).
		SetMethod(sharepolicy.Method(method)).
		SetValue(value).
		SetCreateTime(time.Now())

	if reason != "" {
		builder.SetReason(reason)
	}
	if createdBy != nil {
		builder.SetCreateBy(*createdBy)
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create share policy failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("create share policy failed")
	}

	return entity, nil
}

// ListByShareLinkID lists policies for a share link
func (r *SharePolicyRepo) ListByShareLinkID(ctx context.Context, shareLinkID string) ([]*ent.SharePolicy, error) {
	entities, err := r.entClient.Client().SharePolicy.Query().
		Where(sharepolicy.ShareLinkIDEQ(shareLinkID)).
		Order(ent.Asc(sharepolicy.FieldCreateTime)).
		All(ctx)
	if err != nil {
		r.log.Errorf("list share policies failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("list share policies failed")
	}
	return entities, nil
}

// Delete deletes a share policy by ID
func (r *SharePolicyRepo) Delete(ctx context.Context, id string) error {
	err := r.entClient.Client().SharePolicy.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return sharingV1.ErrorNotFound("share policy not found")
		}
		r.log.Errorf("delete share policy failed: %s", err.Error())
		return sharingV1.ErrorInternalServerError("delete share policy failed")
	}
	return nil
}

// ToProto converts an ent.SharePolicy to sharingV1.SharePolicy
func (r *SharePolicyRepo) ToProto(entity *ent.SharePolicy) *sharingV1.SharePolicy {
	if entity == nil {
		return nil
	}

	proto := &sharingV1.SharePolicy{
		Id:          entity.ID,
		ShareLinkId: entity.ShareLinkID,
		Value:       entity.Value,
		Reason:      entity.Reason,
	}

	switch entity.Type {
	case sharepolicy.TypeBLACKLIST:
		proto.Type = sharingV1.SharePolicyType_SHARE_POLICY_TYPE_BLACKLIST
	case sharepolicy.TypeWHITELIST:
		proto.Type = sharingV1.SharePolicyType_SHARE_POLICY_TYPE_WHITELIST
	}

	switch entity.Method {
	case sharepolicy.MethodIP:
		proto.Method = sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_IP
	case sharepolicy.MethodMAC:
		proto.Method = sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_MAC
	case sharepolicy.MethodREGION:
		proto.Method = sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_REGION
	case sharepolicy.MethodTIME:
		proto.Method = sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_TIME
	case sharepolicy.MethodDEVICE:
		proto.Method = sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_DEVICE
	case sharepolicy.MethodNETWORK:
		proto.Method = sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_NETWORK
	}

	if entity.CreateTime != nil && !entity.CreateTime.IsZero() {
		proto.CreateTime = timestamppb.New(*entity.CreateTime)
	}

	return proto
}
