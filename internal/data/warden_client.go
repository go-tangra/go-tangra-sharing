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
	grpcMD "google.golang.org/grpc/metadata"

	wardenV1 "github.com/go-tangra/go-tangra-warden/gen/go/warden/service/v1"
)

// WardenClient wraps gRPC clients for the Warden service
type WardenClient struct {
	conn          *grpc.ClientConn
	log           *log.Helper
	SecretService wardenV1.WardenSecretServiceClient
}

// NewWardenClient creates a new Warden gRPC client
func NewWardenClient(ctx *bootstrap.Context) (*WardenClient, func(), error) {
	l := ctx.NewLoggerHelper("warden/client/sharing-service")

	endpoint := os.Getenv("WARDEN_GRPC_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9300"
	}

	l.Infof("Connecting to Warden service at: %s", endpoint)

	var dialOpt grpc.DialOption
	creds, err := loadWardenClientTLSCredentials(l)
	if err != nil {
		l.Warnf("Failed to load TLS credentials for Warden, using insecure: %v", err)
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
		l.Errorf("Failed to connect to Warden service: %v", err)
		return nil, func() {}, err
	}

	client := &WardenClient{
		conn:          conn,
		log:           l,
		SecretService: wardenV1.NewWardenSecretServiceClient(conn),
	}

	cleanup := func() {
		if err := conn.Close(); err != nil {
			l.Errorf("Failed to close Warden connection: %v", err)
		}
	}

	l.Info("Warden client initialized successfully")
	return client, cleanup, nil
}

// GetSecret retrieves secret metadata from Warden
func (c *WardenClient) GetSecret(ctx context.Context, tenantID uint32, secretID string) (*wardenV1.Secret, error) {
	if c == nil || c.conn == nil {
		return nil, fmt.Errorf("warden client not available")
	}

	md := grpcMD.New(map[string]string{
		"x-md-global-tenant-id": fmt.Sprintf("%d", tenantID),
	})
	ctx = grpcMD.NewOutgoingContext(ctx, md)

	resp, err := c.SecretService.GetSecret(ctx, &wardenV1.GetSecretRequest{Id: secretID})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret from warden: %w", err)
	}
	return resp.GetSecret(), nil
}

// GetSecretPassword retrieves the password for a secret from Warden
func (c *WardenClient) GetSecretPassword(ctx context.Context, tenantID uint32, secretID string) (string, error) {
	if c == nil || c.conn == nil {
		return "", fmt.Errorf("warden client not available")
	}

	md := grpcMD.New(map[string]string{
		"x-md-global-tenant-id": fmt.Sprintf("%d", tenantID),
	})
	ctx = grpcMD.NewOutgoingContext(ctx, md)

	resp, err := c.SecretService.GetSecretPassword(ctx, &wardenV1.GetSecretPasswordRequest{Id: secretID})
	if err != nil {
		return "", fmt.Errorf("failed to get secret password from warden: %w", err)
	}
	return resp.GetPassword(), nil
}

func loadWardenClientTLSCredentials(l *log.Helper) (credentials.TransportCredentials, error) {
	caCertPath := os.Getenv("WARDEN_CA_CERT_PATH")
	if caCertPath == "" {
		caCertPath = "./data/ca/ca.crt"
	}
	clientCertPath := os.Getenv("WARDEN_CLIENT_CERT_PATH")
	if clientCertPath == "" {
		clientCertPath = "./data/warden/warden.crt"
	}
	clientKeyPath := os.Getenv("WARDEN_CLIENT_KEY_PATH")
	if clientKeyPath == "" {
		clientKeyPath = "./data/warden/warden.key"
	}

	serverName := os.Getenv("WARDEN_SERVER_NAME")
	if serverName == "" {
		serverName = "warden-service"
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
