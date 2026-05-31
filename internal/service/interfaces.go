package service

import (
	"cloudbox/internal/model"
	"context"
)

type FileRepository interface {
	Create(ctx context.Context, file *model.File) error
	FindByID(ctx context.Context, id int64) (*model.File, error)
	FindByIDAndOwner(ctx context.Context, id, ownerID int64) (*model.File, error)
	FindByParentAndOwner(ctx context.Context, parentID, ownerID int64, includeDeleted bool) ([]model.File, error)
	FindByNameAndParent(ctx context.Context, ownerID, parentID int64, name string) (*model.File, error)
	ExistsByName(ctx context.Context, ownerID, parentID int64, name string) bool
	Update(ctx context.Context, file *model.File) error
	SoftDelete(ctx context.Context, id int64) error
	Restore(ctx context.Context, id int64, newParentID *int64, newName string) error
	FindTrash(ctx context.Context, ownerID int64) ([]model.File, error)
	Delete(ctx context.Context, id int64) error
	FindAllDescendants(ctx context.Context, parentID int64) ([]model.File, error)
	BatchUpdateParent(ctx context.Context, fileIDs []int64, newParentID int64) error
	IsAncestor(ctx context.Context, fileID, targetID int64) (bool, error)
	Search(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error)
}

type PhysicalFileRepository interface {
	Create(ctx context.Context, pf *model.PhysicalFile) error
	FindByID(ctx context.Context, id int64) (*model.PhysicalFile, error)
	FindByMD5(ctx context.Context, md5 string) (*model.PhysicalFile, error)
	IncrementRefCount(ctx context.Context, id int64) error
	DecrementRefCount(ctx context.Context, id int64, count int) (int, error)
	UpdateThumbnail(ctx context.Context, id int64, thumbnailPath string) error
	Delete(ctx context.Context, id int64) error
	Update(ctx context.Context, pf *model.PhysicalFile) error
	CalculateUserStorageUsage(ctx context.Context, userID int64) (int64, error)
	CalculateAllUserStorageUsage(ctx context.Context) (map[int64]int64, error)
}

type UserRepository interface {
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	UpdatePassword(ctx context.Context, userID int64, passwordHash string) error
	GetQuota(ctx context.Context, userID int64) (*int64, error)
	SetQuota(ctx context.Context, userID int64, quota *int64) error
}

type Storage interface {
	GenerateFilePath(physicalID int64, ext string) (relative, absolute string)
	ToAbsPath(relative string) string
	ThumbnailPath(physicalID int64) string
	TempChunkDir(uploadID string) string
	EnsureParentDir(path string) error
}

type PasswordHasher interface {
	HashPassword(password string) (string, error)
	CheckPassword(password, hash string) bool
}

type TokenGenerator interface {
	GenerateToken(userID int64, username, role string) (string, error)
}

type ImageProcessor interface {
	ProcessImage(ctx context.Context, physicalID int64) error
}

type ShareRepository interface {
	Create(ctx context.Context, share *model.FileShare) error
	FindByToken(ctx context.Context, token string) (*model.FileShare, error)
	FindByOwner(ctx context.Context, ownerID int64) ([]model.FileShare, error)
	FindByFile(ctx context.Context, fileID int64) ([]model.FileShare, error)
	Revoke(ctx context.Context, id, ownerID int64) error
	IncrementDownloadCount(ctx context.Context, token string) (bool, error)
}
