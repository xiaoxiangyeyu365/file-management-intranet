package handler

import (
	"cloudbox/internal/config"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type StorageHandler struct {
	physicalRepo *repository.PhysicalFileRepository
	userRepo     *repository.UserRepository
}

func NewStorageHandler(physicalRepo *repository.PhysicalFileRepository, userRepo *repository.UserRepository) *StorageHandler {
	return &StorageHandler{physicalRepo: physicalRepo, userRepo: userRepo}
}

func (h *StorageHandler) GetUsage(c *gin.Context) {
	userID := GetUserID(c)

	used, err := h.physicalRepo.CalculateUserStorageUsage(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to calculate storage usage")
		return
	}

	cfg := config.Get()
	effectiveQuota := cfg.Disk.DefaultQuota

	userQuota, err := h.userRepo.GetQuota(c.Request.Context(), userID)
	if err == nil && userQuota != nil {
		effectiveQuota = *userQuota
	}

	var quotaPtr *int64
	if userQuota != nil {
		quotaPtr = userQuota
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"usedBytes":     used,
			"quotaBytes":    quotaPtr,
			"effectiveQuota": effectiveQuota,
		},
	})
}
