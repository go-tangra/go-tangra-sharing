package service

import "github.com/go-tangra/go-tangra-common/grpcx"

var (
	getMetadataValue      = grpcx.GetMetadataValue
	getTenantIDFromContext = grpcx.GetTenantIDFromContext
	getUserIDAsUint32     = grpcx.GetUserIDAsUint32
	getUsernameFromContext = grpcx.GetUsernameFromContext
	getClientIPFromContext = grpcx.GetClientIPFromContext
)
