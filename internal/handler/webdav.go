package handler

import (
	"cloudbox/internal/model"
	"cloudbox/internal/service"
	"cloudbox/internal/util/storage"
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

// cloudFS implements webdav.FileSystem backed by CloudBox services.
type cloudFS struct {
	fileService   *service.FileService
	uploadService *service.UploadService
	audit         service.AuditRecorder
	storage       *storage.StorageManager
}

// NewWebDAVHandler returns an http.Handler that serves WebDAV requests.
func NewWebDAVHandler(fileService *service.FileService, uploadService *service.UploadService, audit service.AuditRecorder, storage *storage.StorageManager) http.Handler {
	fs := &cloudFS{
		fileService:   fileService,
		uploadService: uploadService,
		audit:         audit,
		storage:       storage,
	}

	return &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
	}
}

// resolvePath walks the virtual path and returns the File record.
// Empty path returns a virtual root with ID=0 (IsFolder=true).
func (fs *cloudFS) resolvePath(ctx context.Context, userID int64, webdavPath string) (*model.File, error) {
	webdavPath = path.Clean(webdavPath)
	webdavPath = strings.TrimPrefix(webdavPath, "/")
	if webdavPath == "" || webdavPath == "." {
		return &model.File{ID: 0, Name: "", IsFolder: true, OwnerID: userID}, nil
	}

	segments := strings.Split(webdavPath, "/")
	parentID := int64(0)
	var current *model.File

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		f, err := fs.fileService.FindByName(ctx, userID, parentID, seg)
		if err != nil {
			return nil, os.ErrNotExist
		}
		current = f
		parentID = f.ID
	}

	return current, nil
}

// splitParent splits a path into parent directory path and the final name.
func splitParent(webdavPath string) (parentPath, name string) {
	webdavPath = path.Clean(webdavPath)
	dir := path.Dir(webdavPath)
	base := path.Base(webdavPath)
	if dir == "." {
		dir = ""
	}
	if base == "." {
		base = ""
	}
	if dir == "/" && base != "" {
		dir = ""
	}
	return dir, base
}

// Mkdir implements webdav.FileSystem.
func (fs *cloudFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	userID := GetUserIDFromContext(ctx)
	parentPath, folderName := splitParent(name)

	parent, err := fs.resolvePath(ctx, userID, parentPath)
	if err != nil {
		return os.ErrNotExist
	}

	_, err = fs.fileService.CreateFolder(ctx, userID, parent.ID, folderName)
	if err != nil {
		if err == service.ErrNameConflict {
			return os.ErrExist
		}
		return err
	}
	return nil
}

// Stat implements webdav.FileSystem.
func (fs *cloudFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	userID := GetUserIDFromContext(ctx)
	file, err := fs.resolvePath(ctx, userID, name)
	if err != nil {
		return nil, err
	}
	return &cloudFileInfo{file: file}, nil
}

// RemoveAll implements webdav.FileSystem.
func (fs *cloudFS) RemoveAll(ctx context.Context, name string) error {
	userID := GetUserIDFromContext(ctx)
	file, err := fs.resolvePath(ctx, userID, name)
	if err != nil {
		return err
	}
	if file.ID == 0 {
		return os.ErrPermission // can't delete root
	}

	if err := fs.fileService.MoveToTrash(ctx, userID, file.ID); err != nil {
		if err == service.ErrFileNotFound {
			return os.ErrNotExist
		}
		return err
	}
	return nil
}

// Rename implements webdav.FileSystem.
func (fs *cloudFS) Rename(ctx context.Context, oldName, newName string) error {
	userID := GetUserIDFromContext(ctx)
	srcFile, err := fs.resolvePath(ctx, userID, oldName)
	if err != nil {
		return err
	}
	if srcFile.ID == 0 {
		return os.ErrPermission // can't rename root
	}

	newParentPath, newBaseName := splitParent(newName)
	dstParent, err := fs.resolvePath(ctx, userID, newParentPath)
	if err != nil {
		return os.ErrExist // destination parent not found -> 409
	}

	// Check if destination exists
	target, err := fs.resolvePath(ctx, userID, newName)
	if err == nil {
		// Destination exists
		if target.IsFolder {
			// MOVE into folder: preserve source filename
			newBaseName = srcFile.Name
			dstParent = target
		} else {
			// Destination is a file: soft delete it first
			_ = fs.fileService.MoveToTrash(ctx, userID, target.ID)
		}
	}

	// Same parent? -> Rename. Different parent? -> MoveFiles.
	srcParentID := int64(0)
	if srcFile.ParentID.Valid {
		srcParentID = srcFile.ParentID.Int64
	}

	if srcParentID == dstParent.ID && newBaseName == srcFile.Name {
		return nil // no-op
	}

	if srcParentID == dstParent.ID {
		return fs.fileService.Rename(ctx, userID, srcFile.ID, newBaseName)
	}

	// Cross-folder move
	if err := fs.fileService.MoveFiles(ctx, userID, []int64{srcFile.ID}, dstParent.ID); err != nil {
		if err == service.ErrCircularReference {
			return os.ErrPermission
		}
		if err == service.ErrNameConflict {
			return os.ErrExist
		}
		return err
	}

	// If name also changed, rename after move
	if newBaseName != srcFile.Name {
		return fs.fileService.Rename(ctx, userID, srcFile.ID, newBaseName)
	}
	return nil
}

// cloudFileInfo implements os.FileInfo for a model.File.
type cloudFileInfo struct {
	file *model.File
}

func (fi *cloudFileInfo) Name() string      { return fi.file.Name }
func (fi *cloudFileInfo) Size() int64 {
	if fi.file.Physical != nil {
		return fi.file.Physical.Size
	}
	return 0
}
func (fi *cloudFileInfo) Mode() os.FileMode {
	if fi.file.IsFolder {
		return os.ModeDir | 0755
	}
	return 0644
}
func (fi *cloudFileInfo) ModTime() time.Time { return fi.file.UpdatedAt }
func (fi *cloudFileInfo) IsDir() bool        { return fi.file.IsFolder }
func (fi *cloudFileInfo) Sys() interface{}   { return nil }

// virtualRootInfo is the FileInfo for the virtual root directory.
type virtualRootInfo struct {
	userID int64
}

func (fi *virtualRootInfo) Name() string      { return "" }
func (fi *virtualRootInfo) Size() int64       { return 0 }
func (fi *virtualRootInfo) Mode() os.FileMode { return os.ModeDir | 0755 }
func (fi *virtualRootInfo) ModTime() time.Time { return time.Time{} }
func (fi *virtualRootInfo) IsDir() bool        { return true }
func (fi *virtualRootInfo) Sys() interface{}   { return nil }

// serviceErrorToOSError maps service-layer errors to os errors for webdav.Handler.
func serviceErrorToOSError(err error) error {
	switch err {
	case service.ErrFileNotFound:
		return os.ErrNotExist
	case service.ErrForbidden:
		return os.ErrPermission
	case service.ErrNameConflict:
		return os.ErrExist
	case service.ErrCircularReference:
		return os.ErrPermission
	case service.ErrInvalidTarget:
		return os.ErrNotExist
	case service.ErrIsFolder:
		return os.ErrPermission
	case service.ErrNoPhysicalContent:
		return os.ErrNotExist
	case service.ErrQuotaExceeded:
		return os.ErrPermission
	default:
		return err
	}
}

// OpenFile is a stub -- will be implemented in Task 5.
func (fs *cloudFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	return nil, fmt.Errorf("not implemented")
}

// Copy is a stub -- will be implemented in Task 6.
func (fs *cloudFS) Copy(ctx context.Context, src, dst string, recursive bool) (int, error) {
	return http.StatusNotImplemented, fmt.Errorf("not implemented")
}
