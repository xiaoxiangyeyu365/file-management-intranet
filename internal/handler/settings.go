package handler

import (
	"cloudbox/internal/config"
	"cloudbox/internal/util/response"

	"github.com/gin-gonic/gin"
)

type SettingsHandler struct{}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

// GetSettings returns non-sensitive runtime settings.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	cfg := config.Get()
	response.Success(c, gin.H{
		"disk": gin.H{
			"defaultQuota": cfg.Disk.DefaultQuota,
		},
		"auth": gin.H{
			"registration":      cfg.Auth.Registration,
			"approvalRequired":  cfg.Auth.ApprovalRequired,
		},
		"ai": gin.H{
			"enabled":      cfg.AI.Enabled,
			"model":        cfg.AI.Model,
			"autoDocument": cfg.AI.AutoDocument,
			"autoImage":    cfg.AI.AutoImage,
			"autoVideo":    cfg.AI.AutoVideo,
		},
		"rag": gin.H{
			"enabled": cfg.RAG.Enabled,
		},
		"share": gin.H{
			"tokenLength": cfg.Share.TokenLength,
		},
		"tls": gin.H{
			"enabled": cfg.TLS.Enabled,
			"port":    cfg.TLS.Port,
		},
		"audit": gin.H{
			"retentionDays": cfg.Audit.RetentionDays,
		},
	})
}

// UpdateSettings updates runtime-mutable settings.
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	cfg := config.Get()

	var req struct {
		DefaultQuota     *int64 `json:"defaultQuota"`
		Registration     *bool  `json:"registration"`
		ApprovalRequired *bool  `json:"approvalRequired"`
		TokenLength      *int   `json:"tokenLength"`
		RetentionDays    *int   `json:"retentionDays"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	var changed []string

	if req.DefaultQuota != nil {
		if *req.DefaultQuota < 0 {
			response.BadRequest(c, "defaultQuota must be >= 0")
			return
		}
		cfg.Disk.DefaultQuota = *req.DefaultQuota
		changed = append(changed, "defaultQuota")
	}

	if req.Registration != nil {
		cfg.Auth.Registration = *req.Registration
		changed = append(changed, "registration")
	}

	if req.ApprovalRequired != nil {
		cfg.Auth.ApprovalRequired = *req.ApprovalRequired
		changed = append(changed, "approvalRequired")
	}

	if req.TokenLength != nil {
		if *req.TokenLength < 6 || *req.TokenLength > 64 {
			response.BadRequest(c, "tokenLength must be 6-64")
			return
		}
		cfg.Share.TokenLength = *req.TokenLength
		changed = append(changed, "tokenLength")
	}

	if req.RetentionDays != nil {
		if *req.RetentionDays < 0 || *req.RetentionDays > 3650 {
			response.BadRequest(c, "retentionDays must be 0-3650")
			return
		}
		cfg.Audit.RetentionDays = *req.RetentionDays
		changed = append(changed, "retentionDays")
	}

	response.Success(c, gin.H{
		"message": "settings updated",
		"changed": changed,
	})
}
