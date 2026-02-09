package mail

import (
	"bytes"
	"fmt"
	"html/template"
)

// TemplateData holds the variables available in email templates.
type TemplateData struct {
	SenderName     string
	RecipientEmail string
	ShareLink      string
	Message        string
	ResourceName   string
	ResourceType   string
}

// RenderTemplate renders a Go html/template with the given data.
// Returns the rendered subject and body.
func RenderTemplate(subjectTemplate, htmlBodyTemplate string, data TemplateData) (subject, body string, err error) {
	// Render subject
	subjectTmpl, err := template.New("subject").Parse(subjectTemplate)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse subject template: %w", err)
	}
	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		return "", "", fmt.Errorf("failed to render subject template: %w", err)
	}

	// Render body
	bodyTmpl, err := template.New("body").Parse(htmlBodyTemplate)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse body template: %w", err)
	}
	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
		return "", "", fmt.Errorf("failed to render body template: %w", err)
	}

	return subjectBuf.String(), bodyBuf.String(), nil
}

// DefaultSubjectTemplate is the default email subject template.
const DefaultSubjectTemplate = `{{.SenderName}} shared a {{.ResourceType}} with you`

// DefaultHTMLBodyTemplate is the default email body template.
const DefaultHTMLBodyTemplate = `<!DOCTYPE html>
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
