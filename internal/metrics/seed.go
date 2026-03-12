package metrics

import (
	"context"

	"github.com/go-tangra/go-tangra-sharing/internal/data"
)

// Seed loads initial gauge values from the database.
// Called once at startup so Prometheus has accurate values from the start.
func (c *Collector) Seed(ctx context.Context, statsRepo *data.StatisticsRepo) {
	c.log.Info("Seeding Prometheus metrics from database...")

	linksByStatus, err := statsRepo.GetSharedLinkCountByStatus(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed shared link stats: %v", err)
	} else {
		for status, count := range linksByStatus {
			c.SharedLinksByStatus.WithLabelValues(status).Set(float64(count))
		}
	}

	policyCount, err := statsRepo.GetSharePolicyCount(ctx)
	if err != nil {
		c.log.Errorf("Failed to seed share policy stats: %v", err)
	} else {
		c.SharePoliciesTotal.Set(float64(policyCount))
	}

	c.log.Info("Prometheus metrics seeded successfully")
}
