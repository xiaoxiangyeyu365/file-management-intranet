// internal/handler/share.go
package handler

import (
	"cloudbox/internal/model"
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ShareHandler struct {
	shareService *service.ShareService
	fileService  *service.FileService
}

func NewShareHandler(shareService *service.ShareService, fileService *service.FileService) *ShareHandler {
	return &ShareHandler{
		shareService: shareService,
		fileService:  fileService,
	}
}

// ---------- Public handlers (no JWT) ----------

// GetShareInfo returns public metadata about a share link.
func (h *ShareHandler) GetShareInfo(c *gin.Context) {
	token := c.Param("token")

	share, file, err := h.shareService.GetShareInfo(c.Request.Context(), token)
	if err != nil {
		mapShareError(c, err)
		return
	}

	response.Success(c, gin.H{
		"fileName":    file.Name,
		"fileSize":    getFileSize(file),
		"isFolder":    file.IsFolder,
		"hasPassword": share.PasswordHash.Valid,
		"createdAt":   share.CreatedAt,
		"expiresAt":   share.ExpiresAt,
	})
}

// VerifyShare validates the share password (if any) and returns a download credential.
func (h *ShareHandler) VerifyShare(c *gin.Context) {
	token := c.Param("token")

	var req struct {
		Password string `json:"password"`
	}
	// Binding failure is OK — default to empty password for non-protected shares
	_ = c.ShouldBindJSON(&req)

	cred, err := h.shareService.VerifyOrGetCredential(c.Request.Context(), token, req.Password)
	if err != nil {
		if err == service.ErrWrongSharePassword {
			response.Forbidden(c, "wrong password")
			return
		}
		mapShareError(c, err)
		return
	}

	response.Success(c, gin.H{
		"credential": cred.Credential,
	})
}

// DownloadByShare serves the shared file or folder zip.
func (h *ShareHandler) DownloadByShare(c *gin.Context) {
	token := c.Param("token")
	credential := c.Query("t")

	file, pf, err := h.shareService.DownloadByShare(c.Request.Context(), token, credential)
	if err != nil {
		if err == service.ErrInvalidCredential {
			response.Forbidden(c, "invalid or expired download credential")
			return
		}
		mapShareError(c, err)
		return
	}

	if file.IsFolder {
		encodedName := encodeRFC5987(file.Name)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s.zip", encodedName))
		c.Header("Content-Type", "application/zip")

		if err := h.fileService.StreamFolderZipByID(c.Request.Context(), file.ID, c.Writer); err != nil {
			log.Printf("error streaming folder zip for share: %v", err)
		}
		return
	}

	absPath := h.fileService.GetStorage().ToAbsPath(pf.StoragePath)
	encodedName := encodeRFC5987(file.Name)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedName))
	c.Header("Content-Type", pf.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", pf.Size))
	c.File(absPath)
}

// ---------- Authenticated handlers (JWT required) ----------

// CreateShare creates a new share link for a file.
func (h *ShareHandler) CreateShare(c *gin.Context) {
	userID := GetUserID(c)

	var req struct {
		FileID       int64  `json:"fileId" binding:"required"`
		Password     string `json:"password"`
		ExpiresIn    int64  `json:"expiresIn"`    // seconds, 0 = never
		MaxDownloads int    `json:"maxDownloads"` // 0 = unlimited
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	share, err := h.shareService.CreateShare(c.Request.Context(), userID, req.FileID, req.Password, expiresAt, req.MaxDownloads)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to create share")
		return
	}

	response.Success(c, gin.H{
		"id":        share.ID,
		"token":     share.Token,
		"createdAt": share.CreatedAt,
		"expiresAt": share.ExpiresAt,
	})
}

// ListFileShares returns all shares for a specific file.
func (h *ShareHandler) ListFileShares(c *gin.Context) {
	userID := GetUserID(c)

	fileIDStr := c.Query("fileId")
	if fileIDStr == "" {
		response.BadRequest(c, "fileId parameter is required")
		return
	}
	fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid fileId")
		return
	}

	shares, err := h.shareService.ListFileShares(c.Request.Context(), userID, fileID)
	if err != nil {
		response.InternalError(c, "failed to list shares")
		return
	}

	response.Success(c, gin.H{
		"shares": shares,
	})
}

// ListMyShares returns all shares created by the current user.
func (h *ShareHandler) ListMyShares(c *gin.Context) {
	userID := GetUserID(c)

	shares, err := h.shareService.ListMyShares(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to list shares")
		return
	}

	response.Success(c, gin.H{
		"shares": shares,
	})
}

// RevokeShare revokes a share link.
func (h *ShareHandler) RevokeShare(c *gin.Context) {
	userID := GetUserID(c)
	shareID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	err := h.shareService.RevokeShare(c.Request.Context(), userID, shareID)
	if err != nil {
		response.InternalError(c, "failed to revoke share")
		return
	}

	response.Success(c, nil)
}

// ---------- helpers ----------

// mapShareError maps share-related service errors to HTTP responses.
func mapShareError(c *gin.Context, err error) {
	switch err {
	case service.ErrShareNotFound:
		response.NotFound(c, "share not found")
	case service.ErrShareExpired, service.ErrShareRevoked, service.ErrShareLimitReached:
		c.JSON(http.StatusGone, gin.H{
			"code":    410,
			"message": err.Error(),
		})
	default:
		response.InternalError(c, "internal error")
	}
}

// encodeRFC5987 percent-encodes a string for use in Content-Disposition filename*.
func encodeRFC5987(s string) string {
	var buf []byte
	for _, b := range []byte(s) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '.' || b == '_' || b == '~' {
			buf = append(buf, b)
		} else {
			buf = append(buf, []byte(fmt.Sprintf("%%%02X", b))...)
		}
	}
	return string(buf)
}

// getFileSize returns the physical file size, or 0 if no physical reference.
func getFileSize(file *model.File) int64 {
	if file.Physical != nil {
		return file.Physical.Size
	}
	return 0
}
