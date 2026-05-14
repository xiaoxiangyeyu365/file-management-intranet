// internal/handler/file.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"fmt"
	"log"
	"net/url"
	"strconv"

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
