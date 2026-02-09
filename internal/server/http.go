package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	grpcMD "google.golang.org/grpc/metadata"

	"github.com/go-tangra/go-tangra-sharing/internal/service"

	sharingV1 "github.com/go-tangra/go-tangra-sharing/gen/go/sharing/service/v1"
)

// NewHTTPServer creates an HTTP server for the public sharing endpoint
func NewHTTPServer(
	ctx *bootstrap.Context,
	shareSvc *service.ShareService,
) *kratosHttp.Server {
	l := ctx.NewLoggerHelper("sharing/http")

	addr := os.Getenv("SHARING_HTTP_ADDR")
	if addr == "" {
		addr = "0.0.0.0:9601"
	}

	srv := kratosHttp.NewServer(
		kratosHttp.Address(addr),
	)

	// Register routes
	route := srv.Route("/")

	// CORS preflight
	route.Handle("OPTIONS", "/api/v1/shared/{token}", corsHandler())
	route.Handle("OPTIONS", "/api/v1/shared/{token}/download", corsHandler())

	// Public endpoints (no auth)
	route.GET("/api/v1/shared/{token}", handleViewShared(shareSvc))
	route.GET("/api/v1/shared/{token}/download", handleDownloadShared(shareSvc))

	// Health check
	route.GET("/health", func(ctx kratosHttp.Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	l.Infof("HTTP server listening on %s", addr)
	return srv
}

// handleViewShared returns the shared content as JSON
func handleViewShared(shareSvc *service.ShareService) kratosHttp.HandlerFunc {
	return func(ctx kratosHttp.Context) error {
		setCORSHeaders(ctx)

		token := ctx.Vars().Get("token")
		if token == "" {
			return ctx.JSON(http.StatusBadRequest, errorResponse("token is required"))
		}

		// Extract viewer IP
		viewerIP := ctx.Header().Get("X-Real-IP")
		if viewerIP == "" {
			viewerIP = ctx.Header().Get("X-Forwarded-For")
			if viewerIP != "" {
				// Take first IP if multiple
				if idx := strings.Index(viewerIP, ","); idx > 0 {
					viewerIP = viewerIP[:idx]
				}
			}
		}

		// Inject client IP into gRPC metadata for policy evaluation
		grpcCtx := grpcMD.NewIncomingContext(ctx, grpcMD.Pairs("x-client-ip", viewerIP))

		resp, err := shareSvc.ViewSharedContent(grpcCtx, &sharingV1.ViewSharedContentRequest{
			Token: token,
		})
		if err != nil {
			code, msg := mapShareError(err)
			return ctx.JSON(code, errorResponse(msg))
		}

		return ctx.JSON(http.StatusOK, map[string]interface{}{
			"resourceType": resp.ResourceType.String(),
			"resourceName": resp.ResourceName,
			"password":     resp.Password,
			"fileName":     resp.FileName,
			"mimeType":     resp.MimeType,
		})
	}
}

// handleDownloadShared returns file content with proper headers for document shares
func handleDownloadShared(shareSvc *service.ShareService) kratosHttp.HandlerFunc {
	return func(ctx kratosHttp.Context) error {
		setCORSHeaders(ctx)

		token := ctx.Vars().Get("token")
		if token == "" {
			return ctx.JSON(http.StatusBadRequest, errorResponse("token is required"))
		}

		// Extract viewer IP
		viewerIP := ctx.Header().Get("X-Real-IP")
		if viewerIP == "" {
			viewerIP = ctx.Header().Get("X-Forwarded-For")
			if viewerIP != "" {
				if idx := strings.Index(viewerIP, ","); idx > 0 {
					viewerIP = viewerIP[:idx]
				}
			}
		}

		// Inject client IP into gRPC metadata for policy evaluation
		grpcCtx := grpcMD.NewIncomingContext(ctx, grpcMD.Pairs("x-client-ip", viewerIP))

		resp, err := shareSvc.ViewSharedContent(grpcCtx, &sharingV1.ViewSharedContentRequest{
			Token: token,
		})
		if err != nil {
			code, msg := mapShareError(err)
			return ctx.JSON(code, errorResponse(msg))
		}

		if resp.ResourceType == sharingV1.ResourceType_RESOURCE_TYPE_DOCUMENT && len(resp.FileContent) > 0 {
			mimeType := resp.MimeType
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			fileName := resp.FileName
			if fileName == "" {
				fileName = "download"
			}

			w := ctx.Response()
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
			w.WriteHeader(http.StatusOK)
			_, writeErr := w.Write(resp.FileContent)
			return writeErr
		}

		// For secrets, return JSON
		return ctx.JSON(http.StatusOK, map[string]interface{}{
			"resourceType": resp.ResourceType.String(),
			"resourceName": resp.ResourceName,
			"password":     resp.Password,
		})
	}
}

func setCORSHeaders(ctx kratosHttp.Context) {
	w := ctx.Response()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func corsHandler() kratosHttp.HandlerFunc {
	return func(ctx kratosHttp.Context) error {
		setCORSHeaders(ctx)
		ctx.Response().WriteHeader(http.StatusNoContent)
		return nil
	}
}

func mapShareError(err error) (int, string) {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		return http.StatusNotFound, "share not found or invalid token"
	case strings.Contains(errMsg, "already been viewed"):
		return http.StatusConflict, "this share has already been viewed"
	case strings.Contains(errMsg, "revoked"):
		return http.StatusGone, "this share has been revoked"
	case strings.Contains(errMsg, "access denied") || strings.Contains(errMsg, "blacklist") || strings.Contains(errMsg, "whitelist"):
		return http.StatusForbidden, errMsg
	default:
		return http.StatusInternalServerError, "internal error"
	}
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}

// Ensure json is used (silence import)
var _ = json.Marshal
