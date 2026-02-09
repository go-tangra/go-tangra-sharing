package service

import (
	"context"
	"strconv"

	grpcMD "google.golang.org/grpc/metadata"
)

const (
	mdTenantID = "x-md-global-tenant-id"
	mdUserID   = "x-md-global-user-id"
	mdUsername  = "x-md-global-username"
	mdClientIP = "x-client-ip"
)

func getMetadataValue(ctx context.Context, key string) string {
	md, ok := grpcMD.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func getTenantIDFromContext(ctx context.Context) uint32 {
	tenantStr := getMetadataValue(ctx, mdTenantID)
	if tenantStr == "" {
		return 0
	}
	tenantID, err := strconv.ParseUint(tenantStr, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(tenantID)
}

func getUserIDAsUint32(ctx context.Context) *uint32 {
	userStr := getMetadataValue(ctx, mdUserID)
	if userStr == "" {
		return nil
	}
	userID, err := strconv.ParseUint(userStr, 10, 32)
	if err != nil {
		return nil
	}
	id := uint32(userID)
	return &id
}

func getUsernameFromContext(ctx context.Context) string {
	return getMetadataValue(ctx, mdUsername)
}

func getClientIPFromContext(ctx context.Context) string {
	return getMetadataValue(ctx, mdClientIP)
}
