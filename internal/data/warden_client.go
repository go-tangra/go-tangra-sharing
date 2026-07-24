package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/grpc"
	grpcMD "google.golang.org/grpc/metadata"

	"github.com/go-tangra/go-tangra-common/grpcx"

	wardenV1 "github.com/go-tangra/go-tangra-warden/gen/go/warden/service/v1"
)

// WardenClient wraps gRPC clients for the Warden service.
// It resolves the warden endpoint lazily via ModuleDialer on first use.
type WardenClient struct {
	dialer *grpcx.ModuleDialer
	log    *log.Helper

	once          sync.Once
	conn          *grpc.ClientConn
	SecretService wardenV1.WardenSecretServiceClient
	initErr       error
}

// NewWardenClient creates a new Warden gRPC client that resolves via ModuleDialer.
func NewWardenClient(ctx *bootstrap.Context, dialer *grpcx.ModuleDialer) (*WardenClient, func(), error) {
	l := ctx.NewLoggerHelper("warden/client/sharing-service")

	client := &WardenClient{
		dialer: dialer,
		log:    l,
	}

	cleanup := func() {
		if client.conn != nil {
			if err := client.conn.Close(); err != nil {
				l.Errorf("Failed to close Warden connection: %v", err)
			}
		}
	}

	l.Info("Warden client created (will resolve endpoint on first use)")
	return client, cleanup, nil
}

// resolve lazily connects to the warden service via ModuleDialer.
func (c *WardenClient) resolve() error {
	c.once.Do(func() {
		c.log.Info("Resolving warden module endpoint...")
		conn, err := c.dialer.DialModule(context.Background(), "warden", 30, 5*time.Second)
		if err != nil {
			c.initErr = fmt.Errorf("resolve warden: %w", err)
			c.log.Errorf("Failed to resolve warden: %v", err)
			return
		}
		c.conn = conn
		c.SecretService = wardenV1.NewWardenSecretServiceClient(conn)
		c.log.Info("Warden client connected via ModuleDialer")
	})
	return c.initErr
}

// ForwardMetadataContext builds outgoing gRPC metadata by forwarding relevant headers
// from the incoming context (tenant ID, user ID, username, roles).
func ForwardMetadataContext(ctx context.Context, tenantID uint32) context.Context {
	outMD := grpcMD.New(map[string]string{
		"x-md-global-tenant-id": fmt.Sprintf("%d", tenantID),
	})

	// Forward user identity from incoming context so upstream permission checks pass
	if inMD, ok := grpcMD.FromIncomingContext(ctx); ok {
		// Prefer the gateway-signed tenant-id from the incoming context over the
		// param: it keeps the claim assertion (x-md-global-claims-sig) valid on
		// warden's side under claims-binding enforce, and prevents forwarding a
		// tenant that differs from the one the gateway signed. sig/exp are
		// forwarded so warden re-verifies the original gateway signature.
		for _, key := range []string{"x-md-global-tenant-id", "x-md-global-user-id", "x-md-global-username", "x-md-global-roles", "x-md-global-claims-sig", "x-md-global-claims-exp"} {
			if vals := inMD.Get(key); len(vals) > 0 {
				outMD.Set(key, vals[0])
			}
		}
	}

	return grpcMD.NewOutgoingContext(ctx, outMD)
}

// DetachedMetadataContext extracts gRPC metadata from the incoming request context
// and builds a new outgoing context based on context.Background().
// Use this for async goroutines where the request context will be canceled.
func DetachedMetadataContext(ctx context.Context, tenantID uint32) context.Context {
	outMD := grpcMD.New(map[string]string{
		"x-md-global-tenant-id": fmt.Sprintf("%d", tenantID),
	})

	if inMD, ok := grpcMD.FromIncomingContext(ctx); ok {
		// Prefer the gateway-signed tenant-id from the incoming context over the
		// param: it keeps the claim assertion (x-md-global-claims-sig) valid on
		// warden's side under claims-binding enforce, and prevents forwarding a
		// tenant that differs from the one the gateway signed. sig/exp are
		// forwarded so warden re-verifies the original gateway signature.
		for _, key := range []string{"x-md-global-tenant-id", "x-md-global-user-id", "x-md-global-username", "x-md-global-roles", "x-md-global-claims-sig", "x-md-global-claims-exp"} {
			if vals := inMD.Get(key); len(vals) > 0 {
				outMD.Set(key, vals[0])
			}
		}
	}

	return grpcMD.NewOutgoingContext(context.Background(), outMD)
}

// GetSecret retrieves secret metadata from Warden
func (c *WardenClient) GetSecret(ctx context.Context, tenantID uint32, secretID string) (*wardenV1.Secret, error) {
	if err := c.resolve(); err != nil {
		return nil, err
	}

	ctx = ForwardMetadataContext(ctx, tenantID)

	resp, err := c.SecretService.GetSecret(ctx, &wardenV1.GetSecretRequest{Id: secretID})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret from warden: %w", err)
	}
	return resp.GetSecret(), nil
}

// GetSecretPassword retrieves the password for a secret from Warden
func (c *WardenClient) GetSecretPassword(ctx context.Context, tenantID uint32, secretID string) (string, error) {
	if err := c.resolve(); err != nil {
		return "", err
	}

	ctx = ForwardMetadataContext(ctx, tenantID)

	resp, err := c.SecretService.GetSecretPassword(ctx, &wardenV1.GetSecretPasswordRequest{Id: secretID})
	if err != nil {
		return "", fmt.Errorf("failed to get secret password from warden: %w", err)
	}
	return resp.GetPassword(), nil
}
