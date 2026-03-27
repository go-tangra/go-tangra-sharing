package service

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"google.golang.org/protobuf/types/known/timestamppb"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-common/backup"
	"github.com/go-tangra/go-tangra-common/grpcx"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharedlink"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/sharepolicy"
)

const (
	backupModule        = "sharing"
	backupSchemaVersion = 1
)

var backupMigrations = backup.NewMigrationRegistry(backupModule)

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

func (s *BackupService) ExportBackup(ctx context.Context, req *sharingV1.ExportBackupRequest) (*sharingV1.ExportBackupResponse, error) {
	if !grpcx.IsPlatformAdmin(ctx) {
		return nil, fmt.Errorf("only platform admins can export backups")
	}

	tenantID := grpcx.GetTenantIDFromContext(ctx)
	full := false

	if req.TenantId != nil && *req.TenantId == 0 {
		full = true
		tenantID = 0
	} else if req.TenantId != nil && *req.TenantId != 0 {
		tenantID = *req.TenantId
	}

	client := s.entClient.Client()
	a := backup.NewArchive(backupModule, backupSchemaVersion, tenantID, full)

	// Export shared links
	linkQuery := client.SharedLink.Query()
	if !full {
		linkQuery = linkQuery.Where(sharedlink.TenantID(tenantID))
	}
	links, err := linkQuery.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("export shared links: %w", err)
	}
	if err := backup.SetEntities(a, "sharedLinks", links); err != nil {
		return nil, fmt.Errorf("set shared links: %w", err)
	}

	// Export share policies
	policyQuery := client.SharePolicy.Query()
	if !full {
		policyQuery = policyQuery.Where(sharepolicy.TenantID(tenantID))
	}
	policies, err := policyQuery.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("export share policies: %w", err)
	}
	if err := backup.SetEntities(a, "sharePolicies", policies); err != nil {
		return nil, fmt.Errorf("set share policies: %w", err)
	}

	data, err := backup.Pack(a)
	if err != nil {
		return nil, fmt.Errorf("pack backup: %w", err)
	}

	s.log.Infof("exported backup: module=%s tenant=%d full=%v entities=%v", backupModule, tenantID, full, a.Manifest.EntityCounts)

	return &sharingV1.ExportBackupResponse{
		Data:          data,
		Module:        backupModule,
		Version:       fmt.Sprintf("%d", backupSchemaVersion),
		ExportedAt:    timestamppb.New(a.Manifest.ExportedAt),
		TenantId:      tenantID,
		EntityCounts:  a.Manifest.EntityCounts,
		SchemaVersion: int32(backupSchemaVersion),
	}, nil
}

func (s *BackupService) ImportBackup(ctx context.Context, req *sharingV1.ImportBackupRequest) (*sharingV1.ImportBackupResponse, error) {
	if !grpcx.IsPlatformAdmin(ctx) {
		return nil, fmt.Errorf("only platform admins can import backups")
	}

	tenantID := grpcx.GetTenantIDFromContext(ctx)
	mode := mapSharingRestoreMode(req.GetMode())

	a, err := backup.Unpack(req.GetData())
	if err != nil {
		return nil, fmt.Errorf("unpack backup: %w", err)
	}

	if err := backup.Validate(a, backupModule, backupSchemaVersion); err != nil {
		return nil, err
	}

	sourceVersion := a.Manifest.SchemaVersion
	applied, err := backupMigrations.RunMigrations(a, backupSchemaVersion)
	if err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	if a.Manifest.FullBackup {
		tenantID = 0
	}

	client := s.entClient.Client()
	result := backup.NewRestoreResult(sourceVersion, backupSchemaVersion, applied)

	// Import in FK order: links → policies
	s.importSharedLinks(ctx, client, a, tenantID, a.Manifest.FullBackup, mode, result)
	s.importSharePolicies(ctx, client, a, tenantID, a.Manifest.FullBackup, mode, result)

	s.log.Infof("imported backup: module=%s tenant=%d migrations=%d results=%d",
		backupModule, tenantID, applied, len(result.Results))

	protoResults := make([]*sharingV1.EntityImportResult, len(result.Results))
	for i, r := range result.Results {
		protoResults[i] = &sharingV1.EntityImportResult{
			EntityType: r.EntityType,
			Total:      r.Total,
			Created:    r.Created,
			Updated:    r.Updated,
			Skipped:    r.Skipped,
			Failed:     r.Failed,
		}
	}

	return &sharingV1.ImportBackupResponse{
		Success:           result.Success,
		Results:           protoResults,
		Warnings:          result.Warnings,
		SourceVersion:     int32(result.SourceVersion),
		TargetVersion:     int32(result.TargetVersion),
		MigrationsApplied: int32(result.MigrationsApplied),
	}, nil
}

func mapSharingRestoreMode(m sharingV1.RestoreMode) backup.RestoreMode {
	if m == sharingV1.RestoreMode_RESTORE_MODE_OVERWRITE {
		return backup.RestoreModeOverwrite
	}
	return backup.RestoreModeSkip
}

// --- Import helpers ---

func (s *BackupService) importSharedLinks(ctx context.Context, client *ent.Client, a *backup.Archive, tenantID uint32, full bool, mode backup.RestoreMode, result *backup.RestoreResult) {
	links, err := backup.GetEntities[ent.SharedLink](a, "sharedLinks")
	if err != nil {
		result.AddWarning(fmt.Sprintf("sharedLinks: unmarshal error: %v", err))
		return
	}
	if len(links) == 0 {
		return
	}

	er := backup.EntityResult{EntityType: "sharedLinks", Total: int64(len(links))}

	for _, e := range links {
		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, getErr := client.SharedLink.Get(ctx, e.ID)
		if getErr != nil && !ent.IsNotFound(getErr) {
			result.AddWarning(fmt.Sprintf("sharedLinks: lookup %s: %v", e.ID, getErr))
			er.Failed++
			continue
		}

		if existing != nil {
			if mode == backup.RestoreModeSkip {
				er.Skipped++
				continue
			}
			// Never allow resurrection of viewed or revoked shares
			if existing.Viewed || existing.Revoked {
				result.AddWarning(fmt.Sprintf("sharedLinks: skip %s: cannot overwrite viewed/revoked share", e.ID))
				er.Skipped++
				continue
			}
			builder := client.SharedLink.UpdateOneID(e.ID).
				SetResourceType(e.ResourceType).
				SetResourceID(e.ResourceID).
				SetResourceName(e.ResourceName).
				SetToken(e.Token).
				SetRecipientEmail(e.RecipientEmail).
				SetMessage(e.Message).
				SetViewed(e.Viewed).
				SetNillableViewedAt(e.ViewedAt).
				SetViewedIP(e.ViewedIP).
				SetRevoked(e.Revoked).
				SetNillableCreateBy(e.CreateBy)
			if e.EncryptedContent != nil {
				builder.SetEncryptedContent(*e.EncryptedContent)
			} else {
				builder.ClearEncryptedContent()
			}
			if e.EncryptionNonce != nil {
				builder.SetEncryptionNonce(*e.EncryptionNonce)
			} else {
				builder.ClearEncryptionNonce()
			}
			if _, err := builder.Save(ctx); err != nil {
				result.AddWarning(fmt.Sprintf("sharedLinks: update %s: %v", e.ID, err))
				er.Failed++
				continue
			}
			er.Updated++
		} else {
			// For new creates, strip ciphertext from viewed/revoked shares
			hasContent := e.EncryptedContent != nil
			hasNonce := e.EncryptionNonce != nil
			if e.Viewed || e.Revoked {
				hasContent = false
				hasNonce = false
			}

			createBuilder := client.SharedLink.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetResourceType(e.ResourceType).
				SetResourceID(e.ResourceID).
				SetResourceName(e.ResourceName).
				SetToken(e.Token).
				SetRecipientEmail(e.RecipientEmail).
				SetMessage(e.Message).
				SetViewed(e.Viewed).
				SetNillableViewedAt(e.ViewedAt).
				SetViewedIP(e.ViewedIP).
				SetRevoked(e.Revoked).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime)
			if hasContent {
				createBuilder.SetEncryptedContent(*e.EncryptedContent)
			}
			if hasNonce {
				createBuilder.SetEncryptionNonce(*e.EncryptionNonce)
			}
			if _, err := createBuilder.Save(ctx); err != nil {
				result.AddWarning(fmt.Sprintf("sharedLinks: create %s: %v", e.ID, err))
				er.Failed++
				continue
			}
			er.Created++
		}
	}

	result.AddResult(er)
}

func (s *BackupService) importSharePolicies(ctx context.Context, client *ent.Client, a *backup.Archive, tenantID uint32, full bool, mode backup.RestoreMode, result *backup.RestoreResult) {
	policies, err := backup.GetEntities[ent.SharePolicy](a, "sharePolicies")
	if err != nil {
		result.AddWarning(fmt.Sprintf("sharePolicies: unmarshal error: %v", err))
		return
	}
	if len(policies) == 0 {
		return
	}

	er := backup.EntityResult{EntityType: "sharePolicies", Total: int64(len(policies))}

	for _, e := range policies {
		tid := tenantID
		if full && e.TenantID != nil {
			tid = *e.TenantID
		}

		existing, getErr := client.SharePolicy.Get(ctx, e.ID)
		if getErr != nil && !ent.IsNotFound(getErr) {
			result.AddWarning(fmt.Sprintf("sharePolicies: lookup %s: %v", e.ID, getErr))
			er.Failed++
			continue
		}

		if existing != nil {
			if mode == backup.RestoreModeSkip {
				er.Skipped++
				continue
			}
			if _, err := client.SharePolicy.UpdateOneID(e.ID).
				SetShareLinkID(e.ShareLinkID).
				SetType(e.Type).
				SetMethod(e.Method).
				SetValue(e.Value).
				SetReason(e.Reason).
				SetNillableCreateBy(e.CreateBy).
				Save(ctx); err != nil {
				result.AddWarning(fmt.Sprintf("sharePolicies: update %s: %v", e.ID, err))
				er.Failed++
				continue
			}
			er.Updated++
		} else {
			if _, err := client.SharePolicy.Create().
				SetID(e.ID).
				SetNillableTenantID(&tid).
				SetShareLinkID(e.ShareLinkID).
				SetType(e.Type).
				SetMethod(e.Method).
				SetValue(e.Value).
				SetReason(e.Reason).
				SetNillableCreateBy(e.CreateBy).
				SetNillableCreateTime(e.CreateTime).
				Save(ctx); err != nil {
				result.AddWarning(fmt.Sprintf("sharePolicies: create %s: %v", e.ID, err))
				er.Failed++
				continue
			}
			er.Created++
		}
	}

	result.AddResult(er)
}
