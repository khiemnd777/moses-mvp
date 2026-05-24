package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/core/answer"
)

const publicCitationTTL = 7 * 24 * time.Hour

type publicCitationClaims struct {
	ChunkID string `json:"chunk_id,omitempty"`
	AssetID string `json:"asset_id,omitempty"`
	Exp     int64  `json:"exp"`
}

func (h *Handler) GetPublicCitation(c *fiber.Ctx) error {
	claims, err := h.verifyPublicCitationToken(strings.TrimSpace(c.Params("token")), time.Now())
	if err != nil {
		return respondError(c, fiber.StatusUnauthorized, "invalid_token", "citation link is invalid or expired", nil)
	}
	detail, err := h.publicCitationDetail(c.UserContext(), claims)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, fiber.StatusNotFound, "not_found", "citation not found", nil)
		}
		return respondError(c, fiber.StatusInternalServerError, "citation_error", "failed to load citation", err.Error())
	}
	if strings.EqualFold(c.Query("download"), "1") || strings.EqualFold(c.Query("download"), "true") {
		if detail.Citation.AssetID == "" {
			return respondError(c, fiber.StatusNotFound, "not_found", "citation asset not found", nil)
		}
		asset, err := h.Store.GetDocumentAsset(c.UserContext(), detail.Citation.AssetID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return respondError(c, fiber.StatusNotFound, "not_found", "citation asset not found", nil)
			}
			return respondError(c, fiber.StatusInternalServerError, "db_error", "failed to load citation asset", err.Error())
		}
		return h.sendDocumentAsset(c, asset)
	}
	return h.renderPublicCitationHTML(c, strings.TrimSpace(c.Params("token")), detail)
}

func (h *Handler) publicCitationDetail(ctx context.Context, claims publicCitationClaims) (citationDetailResponse, error) {
	if claims.ChunkID != "" {
		chunks, err := h.Store.GetChunksByIDs(ctx, []string{claims.ChunkID})
		if err != nil {
			return citationDetailResponse{}, err
		}
		if len(chunks) == 0 {
			return citationDetailResponse{}, sql.ErrNoRows
		}
		return h.buildCitationDetailFromChunk(ctx, chunks[0])
	}
	if claims.AssetID == "" {
		return citationDetailResponse{}, sql.ErrNoRows
	}
	asset, err := h.Store.GetDocumentAsset(ctx, claims.AssetID)
	if err != nil {
		return citationDetailResponse{}, err
	}
	content, err := h.Storage.Read(asset.StoragePath)
	if err != nil {
		return citationDetailResponse{}, err
	}
	return citationDetailResponse{
		Citation: answer.Citation{
			AssetID: asset.ID,
			FileURL: "/public/citations/" + h.mustSignPublicCitation(publicCitationClaims{
				AssetID: asset.ID,
				Exp:     time.Now().Add(publicCitationTTL).Unix(),
			}) + "?download=1",
		},
		Content:     content,
		SourceType:  "asset",
		FileName:    asset.FileName,
		ContentType: asset.ContentType,
	}, nil
}

func (h *Handler) renderPublicCitationHTML(c *fiber.Ctx, token string, detail citationDetailResponse) error {
	citation := detail.Citation
	title := firstNonEmpty(citation.CitationLabel, citation.DocumentTitle, citation.LawName, detail.FileName, "Citation")
	downloadURL := ""
	if citation.AssetID != "" {
		downloadURL = "/public/citations/" + url.PathEscape(token) + "?download=1"
	}
	body := strings.Builder{}
	body.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">")
	body.WriteString("<title>")
	body.WriteString(html.EscapeString(title))
	body.WriteString("</title><style>body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;background:#f6f2ea;color:#1f1c17}.wrap{max-width:900px;margin:0 auto;padding:28px}main{background:#fff;border:1px solid #e1d6c8;border-radius:8px;padding:24px;box-shadow:0 12px 30px rgba(31,28,23,.08)}.meta{color:#6f655b;margin:.2rem 0}.content{white-space:pre-wrap;line-height:1.65;margin-top:20px}.button{display:inline-block;margin-top:16px;padding:10px 14px;border-radius:6px;background:#1b6b63;color:#fff;text-decoration:none}</style></head><body><div class=\"wrap\"><main>")
	body.WriteString("<h1>")
	body.WriteString(html.EscapeString(title))
	body.WriteString("</h1>")
	if citation.DocumentNumber != "" {
		body.WriteString("<div class=\"meta\">Số văn bản: ")
		body.WriteString(html.EscapeString(citation.DocumentNumber))
		body.WriteString("</div>")
	}
	if citation.Article != "" || citation.Clause != "" {
		body.WriteString("<div class=\"meta\">")
		if citation.Article != "" {
			body.WriteString("Điều ")
			body.WriteString(html.EscapeString(citation.Article))
		}
		if citation.Clause != "" {
			if citation.Article != "" {
				body.WriteString(", ")
			}
			body.WriteString("Khoản ")
			body.WriteString(html.EscapeString(citation.Clause))
		}
		body.WriteString("</div>")
	}
	if detail.FileName != "" {
		body.WriteString("<div class=\"meta\">File: ")
		body.WriteString(html.EscapeString(detail.FileName))
		body.WriteString("</div>")
	}
	if downloadURL != "" {
		body.WriteString("<a class=\"button\" href=\"")
		body.WriteString(html.EscapeString(downloadURL))
		body.WriteString("\">Tải tài liệu gốc</a>")
	}
	body.WriteString("<div class=\"content\">")
	body.WriteString(html.EscapeString(detail.Content))
	body.WriteString("</div></main></div></body></html>")
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	c.Set(fiber.HeaderCacheControl, "private, max-age=300")
	return c.SendString(body.String())
}

func (h *Handler) buildPublicCitationURL(citation answer.Citation) string {
	baseURL := strings.TrimRight(strings.TrimSpace(h.PublicBaseURL), "/")
	if baseURL == "" || strings.TrimSpace(h.SigningSecret) == "" {
		return ""
	}
	claims := publicCitationClaims{
		ChunkID: firstNonEmpty(citation.ChunkID, citation.ID),
		AssetID: citation.AssetID,
		Exp:     time.Now().Add(publicCitationTTL).Unix(),
	}
	if claims.ChunkID == "" && claims.AssetID == "" {
		return ""
	}
	token, err := h.signPublicCitation(claims)
	if err != nil {
		return ""
	}
	return baseURL + "/public/citations/" + url.PathEscape(token)
}

func (h *Handler) mustSignPublicCitation(claims publicCitationClaims) string {
	token, err := h.signPublicCitation(claims)
	if err != nil {
		return ""
	}
	return token
}

func (h *Handler) signPublicCitation(claims publicCitationClaims) (string, error) {
	if strings.TrimSpace(h.SigningSecret) == "" {
		return "", errors.New("missing signing secret")
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	sig := hmac.New(sha256.New, []byte(h.SigningSecret))
	_, _ = sig.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(sig.Sum(nil)), nil
}

func (h *Handler) verifyPublicCitationToken(token string, now time.Time) (publicCitationClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || strings.TrimSpace(h.SigningSecret) == "" {
		return publicCitationClaims{}, errors.New("invalid citation token")
	}
	sig := hmac.New(sha256.New, []byte(h.SigningSecret))
	_, _ = sig.Write([]byte(parts[0]))
	expected := sig.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return publicCitationClaims{}, errors.New("invalid citation signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return publicCitationClaims{}, err
	}
	var claims publicCitationClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return publicCitationClaims{}, err
	}
	if claims.Exp <= 0 || now.Unix() > claims.Exp {
		return publicCitationClaims{}, errors.New("citation token expired")
	}
	if claims.ChunkID == "" && claims.AssetID == "" {
		return publicCitationClaims{}, errors.New("citation token has no target")
	}
	return claims, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
