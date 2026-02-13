package data

import (
	"context"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	entBootstrap "github.com/tx7do/kratos-bootstrap/database/ent"

	"github.com/go-tangra/go-tangra-sharing/internal/data/ent"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/emailtemplate"
	"github.com/go-tangra/go-tangra-sharing/internal/data/ent/migrate"
	"github.com/go-tangra/go-tangra-sharing/pkg/mail"

	_ "github.com/go-tangra/go-tangra-sharing/internal/data/ent/runtime"
)

// NewEntClient creates an Ent ORM database client
func NewEntClient(ctx *bootstrap.Context) (*entCrud.EntClient[*ent.Client], func(), error) {
	l := ctx.NewLoggerHelper("ent/data/sharing-service")

	cfg := ctx.GetConfig()
	if cfg == nil || cfg.Data == nil {
		l.Fatalf("failed getting config")
		return nil, func() {}, nil
	}

	cli := entBootstrap.NewEntClient(cfg, func(drv *sql.Driver) *ent.Client {
		client := ent.NewClient(
			ent.Driver(drv),
			ent.Log(func(a ...any) {
				l.Info(a...)
			}),
		)
		if client == nil {
			l.Fatalf("failed creating ent client")
			return nil
		}

		// Run database migrations
		if cfg.Data.Database.GetMigrate() {
			if err := client.Schema.Create(context.Background(), migrate.WithForeignKeys(true)); err != nil {
				l.Fatalf("failed creating schema resources: %v", err)
			}
		}

		// Seed default email template
		seedDefaultEmailTemplate(client, l)

		return client
	})

	return cli, func() {
		if err := cli.Close(); err != nil {
			l.Error(err)
		}
	}, nil
}

const defaultTemplateID = "00000000-0000-0000-0000-000000000001"
const defaultTemplateName = "Default Sharing Template"

// seedDefaultEmailTemplate creates the default email template if it doesn't exist.
func seedDefaultEmailTemplate(client *ent.Client, l *log.Helper) {
	ctx := context.Background()

	exists, err := client.EmailTemplate.Query().
		Where(emailtemplate.IDEQ(defaultTemplateID)).
		Exist(ctx)
	if err != nil {
		l.Warnf("Failed to check for default email template: %v", err)
		return
	}
	if exists {
		return
	}

	_, err = client.EmailTemplate.Create().
		SetID(defaultTemplateID).
		SetTenantID(0).
		SetName(defaultTemplateName).
		SetSubject(mail.DefaultSubjectTemplate).
		SetHTMLBody(mail.DefaultHTMLBodyTemplate).
		SetIsDefault(true).
		SetCreateTime(time.Now()).
		Save(ctx)
	if err != nil {
		l.Warnf("Failed to seed default email template: %v", err)
		return
	}

	l.Info("Seeded default email template")
}
