package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/go-tangra/go-tangra-sharing/internal/data"
	"github.com/go-tangra/go-tangra-sharing/pkg/mail"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// TemplateService implements the SharingTemplateService gRPC service
type TemplateService struct {
	sharingV1.UnimplementedSharingTemplateServiceServer

	log          *log.Helper
	templateRepo *data.EmailTemplateRepo
}

// NewTemplateService creates a new TemplateService
func NewTemplateService(
	ctx *bootstrap.Context,
	templateRepo *data.EmailTemplateRepo,
) *TemplateService {
	return &TemplateService{
		log:          ctx.NewLoggerHelper("sharing/service/template"),
		templateRepo: templateRepo,
	}
}

// CreateTemplate creates a new email template
func (s *TemplateService) CreateTemplate(ctx context.Context, req *sharingV1.CreateTemplateRequest) (*sharingV1.CreateTemplateResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	createdBy := getUserIDAsUint32(ctx)

	// Validate template by trying to render it
	_, _, err := mail.RenderTemplate(req.Subject, req.HtmlBody, mail.TemplateData{
		SenderName:   "Test User",
		ResourceName: "Test Resource",
		ResourceType: "SECRET",
		ShareLink:    "https://example.com/shared/test",
	})
	if err != nil {
		return nil, sharingV1.ErrorInvalidTemplate("invalid template: %v", err)
	}

	entity, err := s.templateRepo.Create(ctx, tenantID, req.Name, req.Subject, req.HtmlBody, req.IsDefault, createdBy)
	if err != nil {
		return nil, err
	}

	return &sharingV1.CreateTemplateResponse{
		Template: s.templateRepo.ToProto(entity),
	}, nil
}

// GetTemplate retrieves a template by ID
func (s *TemplateService) GetTemplate(ctx context.Context, req *sharingV1.GetTemplateRequest) (*sharingV1.GetTemplateResponse, error) {
	entity, err := s.templateRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, sharingV1.ErrorTemplateNotFound("template not found")
	}

	return &sharingV1.GetTemplateResponse{
		Template: s.templateRepo.ToProto(entity),
	}, nil
}

// ListTemplates lists templates for the current tenant
func (s *TemplateService) ListTemplates(ctx context.Context, req *sharingV1.ListTemplatesRequest) (*sharingV1.ListTemplatesResponse, error) {
	tenantID := getTenantIDFromContext(ctx)

	var page, pageSize uint32
	if req.Page != nil {
		page = *req.Page
	}
	if req.PageSize != nil {
		pageSize = *req.PageSize
	}

	entities, total, err := s.templateRepo.ListByTenant(ctx, tenantID, page, pageSize)
	if err != nil {
		return nil, err
	}

	templates := make([]*sharingV1.EmailTemplate, 0, len(entities))
	for _, e := range entities {
		templates = append(templates, s.templateRepo.ToProto(e))
	}

	return &sharingV1.ListTemplatesResponse{
		Templates: templates,
		Total:     uint32(total),
	}, nil
}

// UpdateTemplate updates an email template
func (s *TemplateService) UpdateTemplate(ctx context.Context, req *sharingV1.UpdateTemplateRequest) (*sharingV1.UpdateTemplateResponse, error) {
	tenantID := getTenantIDFromContext(ctx)
	updatedBy := getUserIDAsUint32(ctx)

	// Validate if template content is being updated
	subjectTmpl := ""
	bodyTmpl := ""
	if req.Subject != nil {
		subjectTmpl = *req.Subject
	}
	if req.HtmlBody != nil {
		bodyTmpl = *req.HtmlBody
	}
	if subjectTmpl != "" || bodyTmpl != "" {
		// Load existing template for validation
		existing, err := s.templateRepo.GetByID(ctx, req.Id)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, sharingV1.ErrorTemplateNotFound("template not found")
		}
		if subjectTmpl == "" {
			subjectTmpl = existing.Subject
		}
		if bodyTmpl == "" {
			bodyTmpl = existing.HTMLBody
		}

		_, _, err = mail.RenderTemplate(subjectTmpl, bodyTmpl, mail.TemplateData{
			SenderName:   "Test User",
			ResourceName: "Test Resource",
			ResourceType: "SECRET",
			ShareLink:    "https://example.com/shared/test",
		})
		if err != nil {
			return nil, sharingV1.ErrorInvalidTemplate("invalid template: %v", err)
		}
	}

	entity, err := s.templateRepo.Update(ctx, req.Id, tenantID, req.Name, req.Subject, req.HtmlBody, req.IsDefault, updatedBy)
	if err != nil {
		return nil, err
	}

	return &sharingV1.UpdateTemplateResponse{
		Template: s.templateRepo.ToProto(entity),
	}, nil
}

// DeleteTemplate deletes an email template
func (s *TemplateService) DeleteTemplate(ctx context.Context, req *sharingV1.DeleteTemplateRequest) (*emptypb.Empty, error) {
	if err := s.templateRepo.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// PreviewTemplate renders a template with sample data
func (s *TemplateService) PreviewTemplate(ctx context.Context, req *sharingV1.PreviewTemplateRequest) (*sharingV1.PreviewTemplateResponse, error) {
	sampleData := mail.TemplateData{
		SenderName:     "John Doe",
		RecipientEmail: "recipient@example.com",
		ShareLink:      "https://example.com/shared/abc123def456",
		Message:        "Here is the resource you requested.",
		ResourceName:   "My Secret Credential",
		ResourceType:   "Secret",
	}

	subject, body, err := mail.RenderTemplate(req.Subject, req.HtmlBody, sampleData)
	if err != nil {
		return nil, sharingV1.ErrorInvalidTemplate("invalid template: %v", err)
	}

	return &sharingV1.PreviewTemplateResponse{
		RenderedSubject: subject,
		RenderedBody:    body,
	}, nil
}
