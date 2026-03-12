package data

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/grpc"

	"github.com/go-tangra/go-tangra-common/grpcx"

	paperlessV1 "github.com/go-tangra/go-tangra-paperless/gen/go/paperless/service/v1"
)

// PaperlessClient wraps gRPC clients for the Paperless service.
// It resolves the paperless endpoint lazily via ModuleDialer on first use.
type PaperlessClient struct {
	dialer *grpcx.ModuleDialer
	log    *log.Helper

	once            sync.Once
	conn            *grpc.ClientConn
	DocumentService paperlessV1.PaperlessDocumentServiceClient
	initErr         error
}

// NewPaperlessClient creates a new Paperless gRPC client that resolves via ModuleDialer.
func NewPaperlessClient(ctx *bootstrap.Context, dialer *grpcx.ModuleDialer) (*PaperlessClient, func(), error) {
	l := ctx.NewLoggerHelper("paperless/client/sharing-service")

	client := &PaperlessClient{
		dialer: dialer,
		log:    l,
	}

	cleanup := func() {
		if client.conn != nil {
			if err := client.conn.Close(); err != nil {
				l.Errorf("Failed to close Paperless connection: %v", err)
			}
		}
	}

	l.Info("Paperless client created (will resolve endpoint on first use)")
	return client, cleanup, nil
}

// resolve lazily connects to the paperless service via ModuleDialer.
func (c *PaperlessClient) resolve() error {
	c.once.Do(func() {
		c.log.Info("Resolving paperless module endpoint...")
		conn, err := c.dialer.DialModule(context.Background(), "paperless", 30, 5*time.Second)
		if err != nil {
			c.initErr = fmt.Errorf("resolve paperless: %w", err)
			c.log.Errorf("Failed to resolve paperless: %v", err)
			return
		}
		c.conn = conn
		c.DocumentService = paperlessV1.NewPaperlessDocumentServiceClient(conn)
		c.log.Info("Paperless client connected via ModuleDialer")
	})
	return c.initErr
}

// GetDocument retrieves document metadata from Paperless
func (c *PaperlessClient) GetDocument(ctx context.Context, tenantID uint32, documentID string) (*paperlessV1.Document, error) {
	if err := c.resolve(); err != nil {
		return nil, err
	}

	ctx = ForwardMetadataContext(ctx, tenantID)

	resp, err := c.DocumentService.GetDocument(ctx, &paperlessV1.GetDocumentRequest{Id: documentID})
	if err != nil {
		return nil, fmt.Errorf("failed to get document from paperless: %w", err)
	}
	return resp.GetDocument(), nil
}

// DownloadDocument retrieves document content from Paperless
func (c *PaperlessClient) DownloadDocument(ctx context.Context, tenantID uint32, documentID string) ([]byte, string, string, error) {
	if err := c.resolve(); err != nil {
		return nil, "", "", err
	}

	ctx = ForwardMetadataContext(ctx, tenantID)

	resp, err := c.DocumentService.DownloadDocument(ctx, &paperlessV1.DownloadDocumentRequest{Id: documentID})
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to download document from paperless: %w", err)
	}
	return resp.GetContent(), resp.GetFileName(), resp.GetMimeType(), nil
}
