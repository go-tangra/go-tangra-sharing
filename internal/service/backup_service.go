package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-common/grpcx"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/emailtemplate"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharepolicy"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharedlink"
)

const (
	backupModule  = "sharing"
	backupVersion = "1.0"
)

type BackupService struct {
	sharingV1.UnimplementedBackupServiceServer

	log       *log.Helper
	entClient *entCrud.EntClient[*ent.Client]
}

func NewBackupService(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *BackupService {
	return &BackupService{
		log:       ctx.NewLoggerHelper("sharing/service/backup"),
		entClient: entClient,
	}
}

type backupData struct {
	Module     string          `json:"module"`
	Version    string          `json:"version"`
	ExportedAt time.Time       `json:"exportedAt"`
	TenantID   uint32          `json:"tenantId"`
	FullBackup bool            `json:"fullBackup"`
	Data       backupEntities  `json:"data"`
}

type backupEntities struct {
	EmailTemplates []json.RawMessage `json:"emailTemplates,omitempty"`
	SharedLinks    []json.RawMessage `json:"sharedLinks,omitempty"`
	SharePolicies  []json.RawMessage `json:"sharePolicies,omitempty"`
}

func marshalEntities[T any](entities []*T) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(entities))
	for _, e := range entities {
		b, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, nil
}

func (s *BackupService) ExportBackup(ctx context.Context, req *sharingV1.ExportBackupRequest) (*sharingV1.ExportBackupResponse, error) {
	tenantID := grpcx.GetTenantIDFromContext(ctx)
	full := false

	if grpcx.IsPlatformAdmin(ctx) && req.TenantId != nil && *req.TenantId == 0 {
		full = true
		tenantID = 0
	} else if req.TenantId != nil && *req.TenantId != 0 {
		if grpcx.IsPlatformAdmin(ctx) {
			tenantID = *req.TenantId
		}
	}

	client := s.entClient.Client()
	now := time.Now()

	templates, err := s.exportEmailTemplates(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export email templates: %w", err)
	}
	links, err := s.exportSharedLinks(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export shared links: %w", err)
	}
	policies, err := s.exportSharePolicies(ctx, client, tenantID, full)
	if err != nil {
		return nil, fmt.Errorf("export share policies: %w", err)
	}

	backup := backupData{
		Module:     backupModule,
		Version:    backupVersion,
		ExportedAt: now,
		TenantID:   tenantID,
		FullBackup: full,
		Data: backupEntities{
			EmailTemplates: templates,
			SharedLinks:    links,
			SharePolicies:  policies,
		},
	}

	data, err := json.Marshal(backup)
	if err != nil {
		return nil, fmt.Errorf("marshal backup: %w", err)
	}

	entityCounts := map[string]int64{
		"emailTemplates": int64(len(templates)),
		"sharedLinks":    int64(len(links)),
		"sharePolicies":  int64(len(policies)),
	}

	s.log.Infof("exported backup: module=%s tenant=%d full=%v entities=%v", backupModule, tenantID, full, entityCounts)

	return &sharingV1.ExportBackupResponse{
		Data:         data,
		Module:       backupModule,
		Version:      backupVersion,
		ExportedAt:   timestamppb.New(now),
		TenantId:     tenantID,
		EntityCounts: entityCounts,
	}, nil
}

func (s *BackupService) ImportBackup(ctx context.Context, req *sharingV1.ImportBackupRequest) (*sharingV1.ImportBackupResponse, error) {
	tenantID := grpcx.GetTenantIDFromContext(ctx)
	isPlatformAdmin := grpcx.IsPlatformAdmin(ctx)
	mode := req.GetMode()

	var backup backupData
	if err := json.Unmarshal(req.GetData(), &backup); err != nil {
		return nil, fmt.Errorf("invalid backup data: %w", err)
	}

	if backup.Module != backupModule {
		return nil, fmt.Errorf("backup module mismatch: expected %s, got %s", backupModule, backup.Module)
	}
	if backup.Version != backupVersion {
		return nil, fmt.Errorf("backup version mismatch: expected %s, got %s", backupVersion, backup.Version)
	}

	if backup.FullBackup && !isPlatformAdmin {
		return nil, fmt.Errorf("only platform admins can restore full backups")
	}

	if !isPlatformAdmin || !backup.FullBackup {
		tenantID = grpcx.GetTenantIDFromContext(ctx)
	} else {
		tenantID = 0
	}

	client := s.entClient.Client()
	var results []*sharingV1.EntityImportResult
	var warnings []string

	importFuncs := []struct {
		name  string
		items []json.RawMessage
		fn    func(context.Context, *ent.Client, []json.RawMessage, uint32, bool, sharingV1.RestoreMode) (*sharingV1.EntityImportResult, []string)
	}{
		{"emailTemplates", backup.Data.EmailTemplates, s.importEmailTemplates},
		{"sharedLinks", backup.Data.SharedLinks, s.importSharedLinks},
		{"sharePolicies", backup.Data.SharePolicies, s.importSharePolicies},
	}

	for _, imp := range importFuncs {
		if len(imp.items) == 0 {
			continue
		}
		result, w := imp.fn(ctx, client, imp.items, tenantID, backup.FullBackup, mode)
		if result != nil {
			results = append(results, result)
		}
		warnings = append(warnings, w...)
	}

	s.log.Infof("imported backup: module=%s tenant=%d mode=%v results=%d warnings=%d", backupModule, tenantID, mode, len(results), len(warnings))

	return &sharingV1.ImportBackupResponse{
		Success:  true,
		Results:  results,
		Warnings: warnings,
	}, nil
}

// --- Export helpers ---

func (s *BackupService) exportEmailTemplates(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.EmailTemplate.Query()
	if !full {
		query = query.Where(emailtemplate.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportSharedLinks(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.SharedLink.Query()
	if !full {
		query = query.Where(sharedlink.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

func (s *BackupService) exportSharePolicies(ctx context.Context, client *ent.Client, tenantID uint32, full bool) ([]json.RawMessage, error) {
	query := client.SharePolicy.Query()
	if !full {
		query = query.Where(sharepolicy.TenantID(tenantID))
	}
	entities, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	return marshalEntities(entities)
}

// --- Import helpers ---

func (s *BackupService) importEmailTemplates(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode sharingV1.RestoreMode) (*sharingV1.EntityImportResult, []string) {
	result := &sharingV1.EntityImportResult{EntityType: "emailTemplates", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.EmailTemplate
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("emailTemplates: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.EmailTemplate.Get(ctx, e.ID)
		if existing != nil {
			if mode == sharingV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.EmailTemplate.UpdateOneID(e.ID).
				SetName(e.Name).
				SetSubject(e.Subject).
				SetHTMLBody(e.HTMLBody).
				SetIsDefault(e.IsDefault).
				SetNillableCreateBy(e.CreateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("emailTemplates: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.EmailTemplate.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetName(e.Name).
				SetSubject(e.Subject).
				SetHTMLBody(e.HTMLBody).
				SetIsDefault(e.IsDefault).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("emailTemplates: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importSharedLinks(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode sharingV1.RestoreMode) (*sharingV1.EntityImportResult, []string) {
	result := &sharingV1.EntityImportResult{EntityType: "sharedLinks", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.SharedLink
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("sharedLinks: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.SharedLink.Get(ctx, e.ID)
		if existing != nil {
			if mode == sharingV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.SharedLink.UpdateOneID(e.ID).
				SetResourceType(e.ResourceType).
				SetResourceID(e.ResourceID).
				SetResourceName(e.ResourceName).
				SetToken(e.Token).
				SetEncryptedContent(e.EncryptedContent).
				SetEncryptionNonce(e.EncryptionNonce).
				SetRecipientEmail(e.RecipientEmail).
				SetMessage(e.Message).
				SetNillableTemplateID(e.TemplateID).
				SetViewed(e.Viewed).
				SetNillableViewedAt(e.ViewedAt).
				SetViewedIP(e.ViewedIP).
				SetRevoked(e.Revoked).
				SetNillableCreateBy(e.CreateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("sharedLinks: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.SharedLink.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetResourceType(e.ResourceType).
				SetResourceID(e.ResourceID).
				SetResourceName(e.ResourceName).
				SetToken(e.Token).
				SetEncryptedContent(e.EncryptedContent).
				SetEncryptionNonce(e.EncryptionNonce).
				SetRecipientEmail(e.RecipientEmail).
				SetMessage(e.Message).
				SetNillableTemplateID(e.TemplateID).
				SetViewed(e.Viewed).
				SetNillableViewedAt(e.ViewedAt).
				SetViewedIP(e.ViewedIP).
				SetRevoked(e.Revoked).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("sharedLinks: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}

func (s *BackupService) importSharePolicies(ctx context.Context, client *ent.Client, items []json.RawMessage, tenantID uint32, full bool, mode sharingV1.RestoreMode) (*sharingV1.EntityImportResult, []string) {
	result := &sharingV1.EntityImportResult{EntityType: "sharePolicies", Total: int64(len(items))}
	var warnings []string

	for _, raw := range items {
		var e ent.SharePolicy
		if err := json.Unmarshal(raw, &e); err != nil {
			warnings = append(warnings, fmt.Sprintf("sharePolicies: unmarshal error: %v", err))
			result.Failed++
			continue
		}

		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, _ := client.SharePolicy.Get(ctx, e.ID)
		if existing != nil {
			if mode == sharingV1.RestoreMode_RESTORE_MODE_SKIP {
				result.Skipped++
				continue
			}
			_, err := client.SharePolicy.UpdateOneID(e.ID).
				SetShareLinkID(e.ShareLinkID).
				SetType(e.Type).
				SetMethod(e.Method).
				SetValue(e.Value).
				SetReason(e.Reason).
				SetNillableCreateBy(e.CreateBy).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("sharePolicies: update %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Updated++
		} else {
			_, err := client.SharePolicy.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetShareLinkID(e.ShareLinkID).
				SetType(e.Type).
				SetMethod(e.Method).
				SetValue(e.Value).
				SetReason(e.Reason).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("sharePolicies: create %s: %v", e.ID, err))
				result.Failed++
				continue
			}
			result.Created++
		}
	}

	return result, warnings
}
