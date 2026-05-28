// internal/handler/file.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileService *service.FileService
}

func NewFileHandler(fileService *service.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

type CreateFolderRequest struct {
	Name     string `json:"name" binding:"required"`
	ParentID *int64 `json:"parentId"`
}

type RenameRequest struct {
	Name string `json:"name" binding:"required"`
}

type MoveRequest struct {
	FileIDs        []int64 `json:"fileIds" binding:"required"`
	TargetFolderID int64   `json:"targetFolderId" binding:"required"`
}

// ListFiles godoc
// @Summary 获取文件列表
// @Description 获取指定文件夹下的文件和子文件夹
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param folderId query int false "文件夹ID，0表示根目录"
// @Success 200 {object} map[string]interface{} "文件列表"
// @Router /api/files [get]
func (h *FileHandler) ListFiles(c *gin.Context) {
	userID := GetUserID(c)

	folderIDStr := c.Query("folderId")
	var folderID int64
	if folderIDStr != "" {
		folderID, _ = strconv.ParseInt(folderIDStr, 10, 64)
	}

	files, err := h.fileService.ListFiles(c.Request.Context(), userID, folderID)
	if err != nil {
		response.InternalError(c, "failed to list files")
		return
	}

	response.Success(c, gin.H{
		"files": files,
	})
}

// GetFile godoc
// @Summary 获取文件详情
// @Description 根据文件ID获取文件详细信息
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {object} map[string]interface{} "文件信息"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Router /api/files/{id} [get]
func (h *FileHandler) GetFile(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	file, err := h.fileService.GetFile(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to get file")
		return
	}

	response.Success(c, file)
}

// LookupFile godoc
// @Summary 查找文件
// @Description 根据文件名和父文件夹ID查找文件
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param parentId query int false "父文件夹ID"
// @Param name query string true "文件名"
// @Success 200 {object} map[string]interface{} "文件信息"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Router /api/files/lookup [get]
func (h *FileHandler) LookupFile(c *gin.Context) {
	userID := GetUserID(c)

	parentIDStr := c.Query("parentId")
	name := c.Query("name")

	if name == "" {
		response.BadRequest(c, "name parameter is required")
		return
	}

	var parentID int64
	if parentIDStr != "" {
		parentID, _ = strconv.ParseInt(parentIDStr, 10, 64)
	}

	file, err := h.fileService.FindByName(c.Request.Context(), userID, parentID, name)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to lookup file")
		return
	}

	response.Success(c, file)
}

// SearchFiles godoc
// @Summary 搜索文件
// @Description 根据关键词搜索文件名
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param keyword query string true "搜索关键词"
// @Param folderId query int false "限定文件夹ID"
// @Param sort query string false "排序方式(relevance/name/size/time)"
// @Success 200 {object} map[string]interface{} "搜索结果"
// @Router /api/files/search [get]
func (h *FileHandler) SearchFiles(c *gin.Context) {
	userID := GetUserID(c)
	keyword := c.Query("keyword")

	if keyword == "" {
		response.BadRequest(c, "keyword is required")
		return
	}

	folderIDStr := c.Query("folderId")
	var folderID *int64
	if folderIDStr != "" {
		id, _ := strconv.ParseInt(folderIDStr, 10, 64)
		folderID = &id
	}

	sort := c.DefaultQuery("sort", "relevance")

	files, err := h.fileService.SearchFiles(c.Request.Context(), userID, keyword, folderID, sort)
	if err != nil {
		response.InternalError(c, "failed to search files")
		return
	}

	response.Success(c, gin.H{
		"files": files,
	})
}

// CreateFolder godoc
// @Summary 创建文件夹
// @Description 在指定位置创建新文件夹
// @Tags folders
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateFolderRequest true "文件夹信息"
// @Success 200 {object} map[string]interface{} "创建的文件夹信息"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Router /api/folders [post]
func (h *FileHandler) CreateFolder(c *gin.Context) {
	userID := GetUserID(c)

	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	var parentID int64
	if req.ParentID != nil {
		parentID = *req.ParentID
	}

	folder, err := h.fileService.CreateFolder(c.Request.Context(), userID, parentID, req.Name)
	if err != nil {
		if err == service.ErrNameConflict {
			response.Error(c, 400, "folder name already exists")
			return
		}
		response.InternalError(c, "failed to create folder")
		return
	}

	response.Success(c, folder)
}

// RenameFile godoc
// @Summary 重命名文件
// @Description 修改文件或文件夹的名称
// @Tags files
// @Accept json
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Param request body RenameRequest true "新名称"
// @Success 200 {object} map[string]interface{} "重命名成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Router /api/files/{id} [put]
func (h *FileHandler) RenameFile(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	err := h.fileService.Rename(c.Request.Context(), userID, fileID, req.Name)
	if err != nil {
		if err == service.ErrNameConflict {
			response.Error(c, 400, "file name already exists")
			return
		}
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to rename")
		return
	}

	response.Success(c, nil)
}

// DeleteFile godoc
// @Summary 删除文件
// @Description 将文件移至回收站（软删除）
// @Tags files
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Router /api/files/{id} [delete]
func (h *FileHandler) DeleteFile(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	err := h.fileService.MoveToTrash(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to delete")
		return
	}

	response.Success(c, nil)
}

func (h *FileHandler) MoveFiles(c *gin.Context) {
	userID := GetUserID(c)

	var req MoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	err := h.fileService.MoveFiles(c.Request.Context(), userID, req.FileIDs, req.TargetFolderID)
	if err != nil {
		if err == service.ErrCircularReference {
			response.Error(c, 400, "cannot move to a subfolder")
			return
		}
		if err == service.ErrNameConflict {
			response.Error(c, 400, "file name already exists in target folder")
			return
		}
		if err == service.ErrInvalidTarget {
			response.Error(c, 400, "invalid target folder")
			return
		}
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to move files")
		return
	}

	response.Success(c, nil)
}

func (h *FileHandler) DownloadFile(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	file, pf, err := h.fileService.DownloadFile(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.Error(c, 400, err.Error())
		return
	}

	// RFC 5987 encoding for Chinese filename
	encodedName := url.PathEscape(file.Name)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedName))
	c.Header("Content-Type", pf.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", pf.Size))

	// Get absolute path and serve file
	absPath := h.fileService.GetStorage().ToAbsPath(pf.StoragePath)
	c.File(absPath)
}

func (h *FileHandler) GetThumbnail(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	thumbnailPath, err := h.fileService.GetThumbnail(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		if err == service.ErrNotImage {
			response.Error(c, 400, "file is not an image")
			return
		}
		response.InternalError(c, "failed to get thumbnail")
		return
	}

	c.Header("Content-Type", "image/jpeg")
	c.File(thumbnailPath)
}

func (h *FileHandler) DownloadFolder(c *gin.Context) {
	userID := GetUserID(c)
	folderID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	// Get folder info first for filename
	folder, err := h.fileService.GetFile(c.Request.Context(), userID, folderID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "folder not found")
			return
		}
		response.InternalError(c, "failed to get folder")
		return
	}

	if !folder.IsFolder {
		response.Error(c, 400, "not a folder")
		return
	}

	// RFC 5987 encoding for Chinese filename
	encodedName := url.PathEscape(folder.Name)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s.zip", encodedName))
	c.Header("Content-Type", "application/zip")

	if err := h.fileService.StreamFolderZip(c.Request.Context(), userID, folderID, c.Writer); err != nil {
		// Headers already sent, can only log error
		log.Printf("error streaming folder zip: %v", err)
	}
}

func (h *FileHandler) BatchDownload(c *gin.Context) {
	userID := GetUserID(c)

	idsStr := c.Query("ids")
	if idsStr == "" {
		response.Error(c, 400, "ids parameter is required")
		return
	}

	parts := strings.Split(idsStr, ",")
	var fileIDs []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			response.Error(c, 400, "invalid file id: "+p)
			return
		}
		fileIDs = append(fileIDs, id)
	}

	if len(fileIDs) == 0 {
		response.Error(c, 400, "no valid file ids provided")
		return
	}

	c.Header("Content-Disposition", "attachment; filename*=UTF-8''downloads.zip")
	c.Header("Content-Type", "application/zip")

	if err := h.fileService.StreamBatchZip(c.Request.Context(), userID, fileIDs, c.Writer); err != nil {
		log.Printf("error streaming batch zip: %v", err)
	}
}
