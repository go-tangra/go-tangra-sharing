//go:build wireinject
// +build wireinject

//go:generate go run github.com/google/wire/cmd/wire

package providers

import (
	"github.com/google/wire"

	"github.com/go-tangra/go-tangra-sharing/internal/data"
)

// ProviderSet is the Wire provider set for data layer
var ProviderSet = wire.NewSet(
	data.NewRedisClient,
	data.NewEntClient,
	data.NewRegistrationClient,
	data.NewModuleDialer,
	data.NewWardenClient,
	data.NewPaperlessClient,
	data.NewNotificationClient,
	data.NewSharedLinkRepo,
	data.NewSharePolicyRepo,
)
