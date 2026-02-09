package data

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	paperlessV1 "github.com/go-tangra/go-tangra-paperless/gen/go/paperless/service/v1"
)

// PaperlessClient wraps gRPC clients for the Paperless service
type PaperlessClient struct {
	conn            *grpc.ClientConn
	log             *log.Helper
	DocumentService paperlessV1.PaperlessDocumentServiceClient
}

// NewPaperlessClient creates a new Paperless gRPC client
func NewPaperlessClient(ctx *bootstrap.Context) (*PaperlessClient, func(), error) {
	l := ctx.NewLoggerHelper("paperless/client/sharing-service")

	endpoint := os.Getenv("PAPERLESS_GRPC_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9500"
	}

	l.Infof("Connecting to Paperless service at: %s", endpoint)

	var dialOpt grpc.DialOption
	creds, err := loadPaperlessClientTLSCreds(l)
	if err != nil {
		l.Warnf("Failed to load TLS credentials for Paperless, using insecure: %v", err)
		dialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
	} else {
		dialOpt = grpc.WithTransportCredentials(creds)
	}

	connectParams := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  1 * time.Second,
			Multiplier: 1.5,
			Jitter:     0.2,
			MaxDelay:   30 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}

	keepaliveParams := keepalive.ClientParameters{
		Time:                5 * time.Minute,
		Timeout:             20 * time.Second,
		PermitWithoutStream: false,
	}

	conn, err := grpc.NewClient(
		endpoint,
		dialOpt,
		grpc.WithConnectParams(connectParams),
		grpc.WithKeepaliveParams(keepaliveParams),
		grpc.WithDefaultServiceConfig(`{
			"loadBalancingConfig": [{"round_robin":{}}],
			"methodConfig": [{
				"name": [{"service": ""}],
				"waitForReady": true,
				"retryPolicy": {
					"MaxAttempts": 3,
					"InitialBackoff": "0.5s",
					"MaxBackoff": "5s",
					"BackoffMultiplier": 2,
					"RetryableStatusCodes": ["UNAVAILABLE", "RESOURCE_EXHAUSTED"]
				}
			}]
		}`),
	)
	if err != nil {
		l.Errorf("Failed to connect to Paperless service: %v", err)
		return nil, func() {}, err
	}

	client := &PaperlessClient{
		conn:            conn,
		log:             l,
		DocumentService: paperlessV1.NewPaperlessDocumentServiceClient(conn),
	}

	cleanup := func() {
		if err := conn.Close(); err != nil {
			l.Errorf("Failed to close Paperless connection: %v", err)
		}
	}

	l.Info("Paperless client initialized successfully")
	return client, cleanup, nil
}

// GetDocument retrieves document metadata from Paperless
func (c *PaperlessClient) GetDocument(ctx context.Context, tenantID uint32, documentID string) (*paperlessV1.Document, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("paperless client not available")
	}

	ctx = forwardMetadata(ctx, tenantID)

	resp, err := c.DocumentService.GetDocument(ctx, &paperlessV1.GetDocumentRequest{Id: documentID})
	if err != nil {
		return nil, fmt.Errorf("failed to get document from paperless: %w", err)
	}
	return resp.GetDocument(), nil
}

// DownloadDocument retrieves document content from Paperless
func (c *PaperlessClient) DownloadDocument(ctx context.Context, tenantID uint32, documentID string) ([]byte, string, string, error) {
	if c == nil || c.conn == nil {
		return nil, "", "", fmt.Errorf("paperless client not available")
	}

	ctx = forwardMetadata(ctx, tenantID)

	resp, err := c.DocumentService.DownloadDocument(ctx, &paperlessV1.DownloadDocumentRequest{Id: documentID})
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to download document from paperless: %w", err)
	}
	return resp.GetContent(), resp.GetFileName(), resp.GetMimeType(), nil
}

func loadPaperlessClientTLSCreds(l *log.Helper) (credentials.TransportCredentials, error) {
	caCertPath := os.Getenv("PAPERLESS_CA_CERT_PATH")
	if caCertPath == "" {
		caCertPath = "./data/ca/ca.crt"
	}
	clientCertPath := os.Getenv("PAPERLESS_CLIENT_CERT_PATH")
	if clientCertPath == "" {
		clientCertPath = "./data/paperless/paperless.crt"
	}
	clientKeyPath := os.Getenv("PAPERLESS_CLIENT_KEY_PATH")
	if clientKeyPath == "" {
		clientKeyPath = "./data/paperless/paperless.key"
	}

	serverName := os.Getenv("PAPERLESS_SERVER_NAME")
	if serverName == "" {
		serverName = "paperless-service"
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert from %s: %w", caCertPath, err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert/key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		ServerName:   serverName,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}
