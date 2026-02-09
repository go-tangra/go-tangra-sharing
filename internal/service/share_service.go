package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-tangra/go-tangra-sharing/internal/data"
	"github.com/go-tangra/go-tangra-sharing/pkg/crypto"
	"github.com/go-tangra/go-tangra-sharing/pkg/mail"

	"github.com/go-tangra/go-tangra-lcm/pkg/viewer"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// ShareService implements the SharingShareService gRPC service
type ShareService struct {
	sharingV1.UnimplementedSharingShareServiceServer

	log             *log.Helper
	linkRepo        *data.SharedLinkRepo
	templateRepo    *data.EmailTemplateRepo
	policyRepo      *data.SharePolicyRepo
	wardenClient    *data.WardenClient
	paperlessClient *data.PaperlessClient
	mailSender      *mail.Sender
	encryptionKey   []byte
	appHost         string
}

// NewShareService creates a new ShareService
func NewShareService(
	ctx *bootstrap.Context,
	linkRepo *data.SharedLinkRepo,
	templateRepo *data.EmailTemplateRepo,
	policyRepo *data.SharePolicyRepo,
	wardenClient *data.WardenClient,
	paperlessClient *data.PaperlessClient,
	mailSender *mail.Sender,
) *ShareService {
	l := ctx.NewLoggerHelper("sharing/service/share")

	// Parse encryption key from environment
	keyHex := os.Getenv("SHARING_ENCRYPTION_KEY")
	var key []byte
	if keyHex != "" {
		var err error
		key, err = crypto.ParseEncryptionKey(keyHex)
		if err != nil {
			l.Errorf("Invalid SHARING_ENCRYPTION_KEY, falling back to dev key: %v", err)
			key = make([]byte, 32)
			copy(key, []byte("sharing-dev-key-32-bytes-long!!!"))
		}
	} else {
		l.Warn("SHARING_ENCRYPTION_KEY not set, generating random key (shares will not survive restarts)")
		key = make([]byte, 32)
		// Use a deterministic fallback for dev — in production SHARING_ENCRYPTION_KEY must be set
		copy(key, []byte("sharing-dev-key-32-bytes-long!!!"))
	}

	appHost := os.Getenv("APP_HOST")
	if appHost == "" {
		appHost = "http://localhost:5173"
	}

	return &ShareService{
		log:             l,
		linkRepo:        linkRepo,
		templateRepo:    templateRepo,
		policyRepo:      policyRepo,
		wardenClient:    wardenClient,
		paperlessClient: paperlessClient,
		mailSender:      mailSender,
		encryptionKey:   key,
		appHost:         appHost,
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
			return nil, sharingV1.ErrorWardenUnavailable("failed to fetch secret: %v", err)
		}
		resourceName = secret.GetName()

		// Get password
		password, err := s.wardenClient.GetSecretPassword(ctx, tenantID, req.ResourceId)
		if err != nil {
			s.log.Errorf("Failed to get secret password from warden: %v", err)
			return nil, sharingV1.ErrorWardenUnavailable("failed to fetch secret password: %v", err)
		}
		contentBytes = []byte(password)

	case sharingV1.ResourceType_RESOURCE_TYPE_DOCUMENT:
		resourceTypeStr = "DOCUMENT"
		// Get document metadata
		doc, err := s.paperlessClient.GetDocument(ctx, tenantID, req.ResourceId)
		if err != nil {
			s.log.Errorf("Failed to get document from paperless: %v", err)
			return nil, sharingV1.ErrorPaperlessUnavailable("failed to fetch document: %v", err)
		}
		resourceName = doc.GetName()

		// Download document content
		content, _, _, err := s.paperlessClient.DownloadDocument(ctx, tenantID, req.ResourceId)
		if err != nil {
			s.log.Errorf("Failed to download document from paperless: %v", err)
			return nil, sharingV1.ErrorPaperlessUnavailable("failed to download document: %v", err)
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
	var templateID string
	if req.TemplateId != nil {
		templateID = *req.TemplateId
	}

	entity, err := s.linkRepo.Create(ctx, tenantID, resourceTypeStr, req.ResourceId, resourceName, token, ciphertext, nonce, req.RecipientEmail, req.Message, templateID, createdBy)
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

	// Send email asynchronously
	go func() {
		if sendErr := s.sendShareEmail(tenantID, req.RecipientEmail, senderName, resourceName, resourceTypeStr, req.Message, shareLink, templateID); sendErr != nil {
			s.log.Errorf("Failed to send share email: %v", sendErr)
		}
	}()

	return &sharingV1.CreateShareResponse{
		ShareId:   entity.ID,
		ShareLink: shareLink,
	}, nil
}

// GetShare retrieves a share by ID
func (s *ShareService) GetShare(ctx context.Context, req *sharingV1.GetShareRequest) (*sharingV1.GetShareResponse, error) {
	entity, err := s.linkRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
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
	entity, err := s.linkRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
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

	// Evaluate access policies before decrypting
	policies, err := s.policyRepo.ListByShareLinkID(ctx, entity.ID)
	if err != nil {
		s.log.Warnf("Failed to load share policies: %v", err)
	}
	if len(policies) > 0 {
		clientIP := getClientIPFromContext(ctx)
		if policyErr := EvaluatePolicies(policies, clientIP); policyErr != nil {
			return nil, policyErr
		}
	}

	// Decrypt content
	plaintext, err := crypto.DecryptContent(entity.EncryptedContent, entity.EncryptionNonce, s.encryptionKey)
	if err != nil {
		s.log.Errorf("Failed to decrypt content: %v", err)
		return nil, sharingV1.ErrorEncryptionError("failed to decrypt content")
	}

	// Mark as viewed
	if markErr := s.linkRepo.MarkViewed(ctx, entity.ID, ""); markErr != nil {
		s.log.Warnf("Failed to mark share as viewed: %v", markErr)
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

// sendShareEmail sends the share notification email
func (s *ShareService) sendShareEmail(tenantID uint32, recipientEmail, senderName, resourceName, resourceType, message, shareLink, templateID string) error {
	// Use system viewer context for background goroutine (bypasses ENT privacy checks)
	ctx := viewer.NewSystemViewerContext(context.Background())

	// Try to load template
	var subjectTmpl, bodyTmpl string

	if templateID != "" {
		tmpl, err := s.templateRepo.GetByID(ctx, templateID)
		if err == nil && tmpl != nil {
			subjectTmpl = tmpl.Subject
			bodyTmpl = tmpl.HTMLBody
		}
	}

	if subjectTmpl == "" || bodyTmpl == "" {
		// Try default template
		tmpl, err := s.templateRepo.GetDefault(ctx, tenantID)
		if err == nil && tmpl != nil {
			subjectTmpl = tmpl.Subject
			bodyTmpl = tmpl.HTMLBody
		}
	}

	// Fall back to built-in defaults
	if subjectTmpl == "" {
		subjectTmpl = mail.DefaultSubjectTemplate
	}
	if bodyTmpl == "" {
		bodyTmpl = mail.DefaultHTMLBodyTemplate
	}

	data := mail.TemplateData{
		SenderName:     senderName,
		RecipientEmail: recipientEmail,
		ShareLink:      shareLink,
		Message:        message,
		ResourceName:   resourceName,
		ResourceType:   resourceType,
	}

	subject, body, err := mail.RenderTemplate(subjectTmpl, bodyTmpl, data)
	if err != nil {
		return fmt.Errorf("failed to render email template: %w", err)
	}

	return s.mailSender.Send(recipientEmail, subject, body)
}

// CreateSharePolicy creates a policy restriction for a share link
func (s *ShareService) CreateSharePolicy(ctx context.Context, req *sharingV1.CreateSharePolicyRequest) (*sharingV1.CreateSharePolicyResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	// Verify share link exists
	entity, err := s.linkRepo.GetByID(ctx, req.ShareLinkId)
	if err != nil {
		return nil, err
	}
	if entity == nil {
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
	if err := s.policyRepo.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GetEncryptionKeyHex returns the hex-encoded encryption key (for debugging only)
func (s *ShareService) GetEncryptionKeyHex() string {
	return hex.EncodeToString(s.encryptionKey)
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
