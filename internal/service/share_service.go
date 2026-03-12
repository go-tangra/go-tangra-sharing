package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-tangra/go-tangra-sharing/internal/data"
	"github.com/go-tangra/go-tangra-sharing/pkg/crypto"

	notificationv1 "buf.build/gen/go/go-tangra/notification/protocolbuffers/go/notification/service/v1"
	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

const (
	emailRateLimit  = 10              // max emails per window per tenant
	emailRateWindow = 1 * time.Minute // rate limit window
)

// emailRateLimiter tracks email send counts per tenant within a time window.
type emailRateLimiter struct {
	mu      sync.Mutex
	counts  map[uint32]int
	resetAt time.Time
}

func newEmailRateLimiter() *emailRateLimiter {
	return &emailRateLimiter{
		counts:  make(map[uint32]int),
		resetAt: time.Now().Add(emailRateWindow),
	}
}

func (r *emailRateLimiter) allow(tenantID uint32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.After(r.resetAt) {
		r.counts = make(map[uint32]int)
		r.resetAt = now.Add(emailRateWindow)
	}

	if r.counts[tenantID] >= emailRateLimit {
		return false
	}
	r.counts[tenantID]++
	return true
}

const (
	// notificationTemplateName is the name of the template registered in the notification service.
	notificationTemplateName = "sharing-service-template"
	// notificationChannelName is the name of the channel to use in the notification service.
	notificationChannelName = "Default SMTP"

	// defaultSubjectTemplate is the default email subject template.
	defaultSubjectTemplate = `{{.SenderName}} shared a {{.ResourceType}} with you`

	// defaultHTMLBodyTemplate is the default email body template.
	defaultHTMLBodyTemplate = `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
    .container { max-width: 600px; margin: 0 auto; background: #fff; border-radius: 8px; padding: 40px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
    .header { text-align: center; margin-bottom: 30px; }
    .header h1 { color: #1a1a1a; font-size: 24px; margin: 0; }
    .content { color: #333; line-height: 1.6; }
    .message { background: #f8f9fa; border-left: 4px solid #4f46e5; padding: 15px; margin: 20px 0; border-radius: 0 4px 4px 0; }
    .btn { display: inline-block; background: #4f46e5; color: #fff; padding: 12px 30px; text-decoration: none; border-radius: 6px; font-weight: 600; margin: 20px 0; }
    .btn:hover { background: #4338ca; }
    .footer { text-align: center; color: #999; font-size: 12px; margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; }
    .warning { color: #dc2626; font-size: 13px; margin-top: 15px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>Shared {{.ResourceType}}</h1>
    </div>
    <div class="content">
      <p><strong>{{.SenderName}}</strong> has shared a {{.ResourceType}} with you: <strong>{{.ResourceName}}</strong></p>
      {{if .Message}}
      <div class="message">
        <p>{{.Message}}</p>
      </div>
      {{end}}
      <p style="text-align: center;">
        <a href="{{.ShareLink}}" class="btn">View Shared {{.ResourceType}}</a>
      </p>
      <p class="warning">This link can only be viewed once. After viewing, the content will no longer be accessible.</p>
    </div>
    <div class="footer">
      <p>This email was sent via Go Tangra Sharing</p>
    </div>
  </div>
</body>
</html>`
)

// ShareService implements the SharingShareService gRPC service
type ShareService struct {
	sharingV1.UnimplementedSharingShareServiceServer

	log                *log.Helper
	linkRepo           *data.SharedLinkRepo
	policyRepo         *data.SharePolicyRepo
	wardenClient       *data.WardenClient
	paperlessClient    *data.PaperlessClient
	notificationClient *data.NotificationClient
	encryptionKey      []byte
	appHost            string
	rateLimiter        *emailRateLimiter

	// notifTemplateID is resolved once on first successful email send
	notifTemplateMu  sync.Mutex
	notifTemplateID  string
}

// NewShareService creates a new ShareService
func NewShareService(
	ctx *bootstrap.Context,
	linkRepo *data.SharedLinkRepo,
	policyRepo *data.SharePolicyRepo,
	wardenClient *data.WardenClient,
	paperlessClient *data.PaperlessClient,
	notificationClient *data.NotificationClient,
) *ShareService {
	l := ctx.NewLoggerHelper("sharing/service/share")

	// Parse encryption key from environment (required)
	keyHex := os.Getenv("SHARING_ENCRYPTION_KEY")
	if keyHex == "" {
		l.Errorf("SHARING_ENCRYPTION_KEY environment variable is not set — refusing to start")
		panic("SHARING_ENCRYPTION_KEY must be set")
	}
	key, err := crypto.ParseEncryptionKey(keyHex)
	if err != nil {
		l.Errorf("Invalid SHARING_ENCRYPTION_KEY: %v", err)
		panic("SHARING_ENCRYPTION_KEY is invalid: " + err.Error())
	}

	appHost := os.Getenv("APP_HOST")
	if appHost == "" {
		appHost = "http://localhost:5173"
	}

	return &ShareService{
		log:                l,
		linkRepo:           linkRepo,
		policyRepo:         policyRepo,
		wardenClient:       wardenClient,
		paperlessClient:    paperlessClient,
		notificationClient: notificationClient,
		encryptionKey:      key,
		appHost:            appHost,
		rateLimiter:        newEmailRateLimiter(),
	}
}

// CreateShare creates a new share, encrypts content, stores it, and sends an email
func (s *ShareService) CreateShare(ctx context.Context, req *sharingV1.CreateShareRequest) (*sharingV1.CreateShareResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)
	senderName := getUsernameFromContext(ctx)
	if senderName == "" {
		senderName = "A user"
	}

	// Validate resource type
	if req.ResourceType == sharingV1.ResourceType_RESOURCE_TYPE_UNSPECIFIED {
		return nil, sharingV1.ErrorInvalidResourceType("resource type must be SECRET or DOCUMENT")
	}

	// Fetch content from upstream service
	var contentBytes []byte
	var resourceName string
	var resourceTypeStr string

	switch req.ResourceType {
	case sharingV1.ResourceType_RESOURCE_TYPE_SECRET:
		resourceTypeStr = "SECRET"
		// Get secret metadata
		secret, err := s.wardenClient.GetSecret(ctx, tenantID, req.ResourceId)
		if err != nil {
			s.log.Errorf("Failed to get secret from warden: %v", err)
			return nil, sharingV1.ErrorWardenUnavailable("failed to fetch secret")
		}
		resourceName = secret.GetName()

		// Get password
		password, err := s.wardenClient.GetSecretPassword(ctx, tenantID, req.ResourceId)
		if err != nil {
			s.log.Errorf("Failed to get secret password from warden: %v", err)
			return nil, sharingV1.ErrorWardenUnavailable("failed to fetch secret password")
		}
		contentBytes = []byte(password)

	case sharingV1.ResourceType_RESOURCE_TYPE_DOCUMENT:
		resourceTypeStr = "DOCUMENT"
		// Get document metadata
		doc, err := s.paperlessClient.GetDocument(ctx, tenantID, req.ResourceId)
		if err != nil {
			s.log.Errorf("Failed to get document from paperless: %v", err)
			return nil, sharingV1.ErrorPaperlessUnavailable("failed to fetch document")
		}
		resourceName = doc.GetName()

		// Download document content
		content, _, _, err := s.paperlessClient.DownloadDocument(ctx, tenantID, req.ResourceId)
		if err != nil {
			s.log.Errorf("Failed to download document from paperless: %v", err)
			return nil, sharingV1.ErrorPaperlessUnavailable("failed to download document")
		}
		contentBytes = content

	default:
		return nil, sharingV1.ErrorInvalidResourceType("unsupported resource type")
	}

	// Generate token
	token, err := crypto.GenerateToken()
	if err != nil {
		s.log.Errorf("Failed to generate token: %v", err)
		return nil, sharingV1.ErrorEncryptionError("failed to generate share token")
	}

	// Encrypt content
	ciphertext, nonce, err := crypto.EncryptContent(contentBytes, s.encryptionKey)
	if err != nil {
		s.log.Errorf("Failed to encrypt content: %v", err)
		return nil, sharingV1.ErrorEncryptionError("failed to encrypt content")
	}

	// Store in database
	entity, err := s.linkRepo.Create(ctx, tenantID, resourceTypeStr, req.ResourceId, resourceName, token, ciphertext, nonce, req.RecipientEmail, req.Message, createdBy)
	if err != nil {
		return nil, err
	}

	// Create policies if provided
	if len(req.Policies) > 0 {
		for _, p := range req.Policies {
			pType := policyTypeToString(p.Type)
			pMethod := policyMethodToString(p.Method)
			if pType == "" || pMethod == "" {
				continue
			}
			_, err := s.policyRepo.Create(ctx, tenantID, entity.ID, pType, pMethod, p.Value, p.Reason, createdBy)
			if err != nil {
				s.log.Warnf("Failed to create share policy: %v", err)
			}
		}
	}

	// Build share link
	shareLink := fmt.Sprintf("%s/#/shared/%s", s.appHost, token)

	// Check email rate limit before sending
	if !s.rateLimiter.allow(tenantID) {
		s.log.Warnf("Email rate limit exceeded for tenant %d, skipping email to %s", tenantID, req.RecipientEmail)
	} else {
		// Send email asynchronously with panic recovery.
		// We must NOT derive from the request ctx — gRPC cancels it after the handler returns.
		// Build a detached context that carries the same metadata.
		sendCtx := data.DetachedMetadataContext(ctx, tenantID)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.log.Errorf("Panic in email send goroutine: %v", r)
				}
			}()
			if sendErr := s.sendShareEmail(sendCtx, req.RecipientEmail, senderName, resourceName, resourceTypeStr, req.Message, shareLink); sendErr != nil {
				s.log.Errorf("Failed to send share email: %v", sendErr)
			}
		}()
	}

	return &sharingV1.CreateShareResponse{
		ShareId:   entity.ID,
		ShareLink: shareLink,
	}, nil
}

// GetShare retrieves a share by ID
func (s *ShareService) GetShare(ctx context.Context, req *sharingV1.GetShareRequest) (*sharingV1.GetShareResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	entity, err := s.linkRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil || (entity.TenantID != nil && *entity.TenantID != tenantID) {
		return nil, sharingV1.ErrorShareNotFound("share not found")
	}

	shareProto := s.linkRepo.ToProto(entity)

	// Load policies
	policies, pErr := s.policyRepo.ListByShareLinkID(ctx, entity.ID)
	if pErr == nil && len(policies) > 0 {
		for _, p := range policies {
			shareProto.Policies = append(shareProto.Policies, s.policyRepo.ToProto(p))
		}
	}

	return &sharingV1.GetShareResponse{
		Share: shareProto,
	}, nil
}

// ListShares lists shares for the current tenant
func (s *ShareService) ListShares(ctx context.Context, req *sharingV1.ListSharesRequest) (*sharingV1.ListSharesResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	var page, pageSize uint32
	if req.Page != nil {
		page = *req.Page
	}
	if req.PageSize != nil {
		pageSize = *req.PageSize
	}

	var resourceType *string
	if req.ResourceType != nil && *req.ResourceType != sharingV1.ResourceType_RESOURCE_TYPE_UNSPECIFIED {
		rt := req.ResourceType.String()
		resourceType = &rt
	}

	var recipientEmail *string
	if req.RecipientEmail != nil {
		recipientEmail = req.RecipientEmail
	}

	entities, total, err := s.linkRepo.ListByTenant(ctx, tenantID, resourceType, recipientEmail, page, pageSize)
	if err != nil {
		return nil, err
	}

	shares := make([]*sharingV1.SharedLink, 0, len(entities))
	for _, e := range entities {
		shares = append(shares, s.linkRepo.ToProto(e))
	}

	return &sharingV1.ListSharesResponse{
		Shares: shares,
		Total:  uint32(total),
	}, nil
}

// RevokeShare revokes a shared link
func (s *ShareService) RevokeShare(ctx context.Context, req *sharingV1.RevokeShareRequest) (*emptypb.Empty, error) {
	tenantID := getTenantIDFromContext(ctx)

	entity, err := s.linkRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil || (entity.TenantID != nil && *entity.TenantID != tenantID) {
		return nil, sharingV1.ErrorShareNotFound("share not found")
	}

	if err := s.linkRepo.Revoke(ctx, req.Id); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ViewSharedContent views the content of a shared link (consumes the link)
func (s *ShareService) ViewSharedContent(ctx context.Context, req *sharingV1.ViewSharedContentRequest) (*sharingV1.ViewSharedContentResponse, error) {
	entity, err := s.linkRepo.GetByToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, sharingV1.ErrorShareNotFound("share not found or invalid token")
	}

	if entity.Revoked {
		return nil, sharingV1.ErrorShareRevoked("this share has been revoked")
	}

	if entity.Viewed {
		return nil, sharingV1.ErrorShareAlreadyViewed("this share has already been viewed")
	}

	// Evaluate access policies before decrypting (fail-closed: deny if policies can't be loaded)
	policies, err := s.policyRepo.ListByShareLinkID(ctx, entity.ID)
	if err != nil {
		s.log.Errorf("Failed to load share policies, denying access: %v", err)
		return nil, sharingV1.ErrorShareAccessDenied("unable to verify access policies")
	}

	clientIP := getClientIPFromContext(ctx)
	if len(policies) > 0 {
		if policyErr := EvaluatePolicies(policies, clientIP); policyErr != nil {
			return nil, policyErr
		}
	}

	// Decrypt content
	if entity.EncryptedContent == nil || entity.EncryptionNonce == nil {
		return nil, sharingV1.ErrorEncryptionError("share content is no longer available")
	}
	plaintext, err := crypto.DecryptContent(*entity.EncryptedContent, *entity.EncryptionNonce, s.encryptionKey)
	if err != nil {
		s.log.Errorf("Failed to decrypt content: %v", err)
		return nil, sharingV1.ErrorEncryptionError("failed to decrypt content")
	}

	// Atomically mark as viewed (WHERE viewed=false) — prevents TOCTOU race.
	// Only return decrypted content if the atomic mark succeeds.
	if markErr := s.linkRepo.MarkViewed(ctx, entity.ID, clientIP); markErr != nil {
		s.log.Errorf("Failed to consume share: %v", markErr)
		return nil, markErr
	}

	resp := &sharingV1.ViewSharedContentResponse{
		ResourceName: entity.ResourceName,
	}

	switch entity.ResourceType {
	case "SECRET":
		resp.ResourceType = sharingV1.ResourceType_RESOURCE_TYPE_SECRET
		resp.Password = string(plaintext)
	case "DOCUMENT":
		resp.ResourceType = sharingV1.ResourceType_RESOURCE_TYPE_DOCUMENT
		resp.FileContent = plaintext
		resp.FileName = entity.ResourceName
		resp.MimeType = "application/octet-stream"
	}

	return resp, nil
}

// ensureNotificationTemplate resolves (or creates) the sharing template in the notification service.
// Called lazily on first email send. Uses platform admin context (tenant 0) because notification
// channels and templates are platform-level resources, not tenant-scoped.
func (s *ShareService) ensureNotificationTemplate(ctx context.Context) (string, error) {
	s.notifTemplateMu.Lock()
	defer s.notifTemplateMu.Unlock()

	// Already resolved successfully — return cached ID
	if s.notifTemplateID != "" {
		return s.notifTemplateID, nil
	}

	s.log.Info("Resolving notification template for sharing service...")

	// Use platform admin context (tenant 0) to access platform-level notification resources
	platformCtx := data.DetachedMetadataContext(ctx, 0)

	// Search for existing template
	tmpl, err := s.notificationClient.FindTemplateByName(platformCtx, notificationTemplateName)
	if err != nil {
		return "", fmt.Errorf("search notification template: %w", err)
	}
	if tmpl != nil {
		s.notifTemplateID = tmpl.GetId()
		s.log.Infof("Found existing notification template: %s", s.notifTemplateID)
		return s.notifTemplateID, nil
	}

	// Template not found — find the channel and create the template
	channelID, err := s.notificationClient.FindChannelByName(platformCtx, notificationChannelName)
	if err != nil {
		return "", fmt.Errorf("find channel %q: %w", notificationChannelName, err)
	}

	createReq := &notificationv1.CreateTemplateRequest{
		Name:      notificationTemplateName,
		ChannelId: channelID,
		Subject:   defaultSubjectTemplate,
		Body:      defaultHTMLBodyTemplate,
		Variables: "SenderName,RecipientEmail,ShareLink,Message,ResourceName,ResourceType",
		IsDefault: false,
	}
	created, err := s.notificationClient.CreateTemplate(platformCtx, createReq)
	if err != nil {
		return "", fmt.Errorf("create notification template: %w", err)
	}
	s.notifTemplateID = created.GetId()
	s.log.Infof("Created notification template: %s", s.notifTemplateID)
	return s.notifTemplateID, nil
}

// sendShareEmail sends the share notification email via the notification service.
func (s *ShareService) sendShareEmail(ctx context.Context, recipientEmail, senderName, resourceName, resourceType, message, shareLink string) error {
	templateID, err := s.ensureNotificationTemplate(ctx)
	if err != nil {
		return fmt.Errorf("ensure notification template: %w", err)
	}

	variables := map[string]string{
		"SenderName":     senderName,
		"RecipientEmail": recipientEmail,
		"ShareLink":      shareLink,
		"Message":        message,
		"ResourceName":   resourceName,
		"ResourceType":   resourceType,
	}

	// Use platform admin context (tenant 0) for notification service calls
	platformCtx := data.DetachedMetadataContext(ctx, 0)
	_, err = s.notificationClient.SendNotification(platformCtx, templateID, recipientEmail, variables)
	if err != nil {
		return fmt.Errorf("send notification: %w", err)
	}

	return nil
}

// CreateSharePolicy creates a policy restriction for a share link
func (s *ShareService) CreateSharePolicy(ctx context.Context, req *sharingV1.CreateSharePolicyRequest) (*sharingV1.CreateSharePolicyResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	// Verify share link exists and belongs to the caller's tenant
	entity, err := s.linkRepo.GetByID(ctx, req.ShareLinkId)
	if err != nil {
		return nil, err
	}
	if entity == nil || (entity.TenantID != nil && *entity.TenantID != tenantID) {
		return nil, sharingV1.ErrorShareNotFound("share not found")
	}

	pType := policyTypeToString(req.Type)
	pMethod := policyMethodToString(req.Method)

	policy, err := s.policyRepo.Create(ctx, tenantID, req.ShareLinkId, pType, pMethod, req.Value, req.Reason, createdBy)
	if err != nil {
		return nil, err
	}

	return &sharingV1.CreateSharePolicyResponse{
		Policy: s.policyRepo.ToProto(policy),
	}, nil
}

// ListSharePolicies lists policy restrictions for a share link
func (s *ShareService) ListSharePolicies(ctx context.Context, req *sharingV1.ListSharePoliciesRequest) (*sharingV1.ListSharePoliciesResponse, error) {
	policies, err := s.policyRepo.ListByShareLinkID(ctx, req.ShareLinkId)
	if err != nil {
		return nil, err
	}

	result := make([]*sharingV1.SharePolicy, 0, len(policies))
	for _, p := range policies {
		result = append(result, s.policyRepo.ToProto(p))
	}

	return &sharingV1.ListSharePoliciesResponse{
		Policies: result,
	}, nil
}

// DeleteSharePolicy deletes a policy restriction
func (s *ShareService) DeleteSharePolicy(ctx context.Context, req *sharingV1.DeleteSharePolicyRequest) (*emptypb.Empty, error) {
	tenantID := getTenantIDFromContext(ctx)

	// Verify policy exists and belongs to the caller's tenant
	policy, err := s.policyRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if policy == nil || (policy.TenantID != nil && *policy.TenantID != tenantID) {
		return nil, sharingV1.ErrorNotFound("share policy not found")
	}

	if err := s.policyRepo.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// policyTypeToString converts proto enum to ent enum string
func policyTypeToString(t sharingV1.SharePolicyType) string {
	switch t {
	case sharingV1.SharePolicyType_SHARE_POLICY_TYPE_BLACKLIST:
		return "BLACKLIST"
	case sharingV1.SharePolicyType_SHARE_POLICY_TYPE_WHITELIST:
		return "WHITELIST"
	default:
		return ""
	}
}

// policyMethodToString converts proto enum to ent enum string
func policyMethodToString(m sharingV1.SharePolicyMethod) string {
	switch m {
	case sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_IP:
		return "IP"
	case sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_MAC:
		return "MAC"
	case sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_REGION:
		return "REGION"
	case sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_TIME:
		return "TIME"
	case sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_DEVICE:
		return "DEVICE"
	case sharingV1.SharePolicyMethod_SHARE_POLICY_METHOD_NETWORK:
		return "NETWORK"
	default:
		return ""
	}
}
