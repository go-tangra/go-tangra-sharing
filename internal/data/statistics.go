package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	entCrud "github.com/tx7do/go-crud/entgo"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-sharing/internal/data/ent"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharedlink"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// StatisticsRepo provides methods for collecting Sharing statistics
type StatisticsRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewStatisticsRepo creates a new StatisticsRepo
func NewStatisticsRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *StatisticsRepo {
	return &StatisticsRepo{
		entClient: entClient,
		log:       ctx.NewLoggerHelper("sharing/statistics/repo"),
	}
}

// GetSharedLinkCount returns the total number of shared links across all tenants.
func (r *StatisticsRepo) GetSharedLinkCount(ctx context.Context) (int, error) {
	count, err := r.entClient.Client().SharedLink.Query().Count(ctx)
	if err != nil {
		r.log.Errorf("get shared link count failed: %s", err.Error())
		return 0, sharingV1.ErrorInternalServerError("get statistics failed")
	}
	return count, nil
}

// GetSharedLinkCountByStatus returns the count of shared links grouped by status.
// Status is derived from boolean fields: "active" (not viewed, not revoked),
// "viewed" (viewed, not revoked), "revoked" (revoked regardless of viewed).
func (r *StatisticsRepo) GetSharedLinkCountByStatus(ctx context.Context) (map[string]int, error) {
	result := make(map[string]int)

	// Revoked links (revoked=true, regardless of viewed)
	revokedCount, err := r.entClient.Client().SharedLink.Query().
		Where(sharedlink.RevokedEQ(true)).
		Count(ctx)
	if err != nil {
		r.log.Errorf("get revoked shared link count failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("get statistics failed")
	}
	if revokedCount > 0 {
		result["revoked"] = revokedCount
	}

	// Viewed links (viewed=true, revoked=false)
	viewedCount, err := r.entClient.Client().SharedLink.Query().
		Where(
			sharedlink.ViewedEQ(true),
			sharedlink.RevokedEQ(false),
		).
		Count(ctx)
	if err != nil {
		r.log.Errorf("get viewed shared link count failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("get statistics failed")
	}
	if viewedCount > 0 {
		result["viewed"] = viewedCount
	}

	// Active links (viewed=false, revoked=false)
	activeCount, err := r.entClient.Client().SharedLink.Query().
		Where(
			sharedlink.ViewedEQ(false),
			sharedlink.RevokedEQ(false),
		).
		Count(ctx)
	if err != nil {
		r.log.Errorf("get active shared link count failed: %s", err.Error())
		return nil, sharingV1.ErrorInternalServerError("get statistics failed")
	}
	if activeCount > 0 {
		result["active"] = activeCount
	}

	return result, nil
}

// GetSharePolicyCount returns the total number of share policies across all tenants.
func (r *StatisticsRepo) GetSharePolicyCount(ctx context.Context) (int, error) {
	count, err := r.entClient.Client().SharePolicy.Query().Count(ctx)
	if err != nil {
		r.log.Errorf("get share policy count failed: %s", err.Error())
		return 0, sharingV1.ErrorInternalServerError("get statistics failed")
	}
	return count, nil
}
