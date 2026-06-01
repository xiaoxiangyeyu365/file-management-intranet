package service

import (
	"cloudbox/internal/model"
	"context"
)

type noopAuditRecorder struct{}

func (noopAuditRecorder) Record(ctx context.Context, action, targetType string, targetID int64, targetName, detail string) {}

var noopAudit = noopAuditRecorder{}

type mockFileRepo struct {
	createFn              func(ctx context.Context, file *model.File) error
	findByIDFn            func(ctx context.Context, id int64) (*model.File, error)
	findByIDAndOwnerFn    func(ctx context.Context, id, ownerID int64) (*model.File, error)
	findByParentAndOwnerFn func(ctx context.Context, parentID, ownerID int64, includeDeleted bool) ([]model.File, error)
	findByNameAndParentFn func(ctx context.Context, ownerID, parentID int64, name string) (*model.File, error)
	existsByNameFn        func(ctx context.Context, ownerID, parentID int64, name string) bool
	updateFn              func(ctx context.Context, file *model.File) error
	softDeleteFn          func(ctx context.Context, id int64) error
	restoreFn             func(ctx context.Context, id int64, newParentID *int64, newName string) error
	findTrashFn           func(ctx context.Context, ownerID int64) ([]model.File, error)
	deleteFn              func(ctx context.Context, id int64) error
	findAllDescendantsFn  func(ctx context.Context, parentID int64) ([]model.File, error)
	batchUpdateParentFn   func(ctx context.Context, fileIDs []int64, newParentID int64) error
	isAncestorFn          func(ctx context.Context, fileID, targetID int64) (bool, error)
	searchFn              func(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error)
}

func (m *mockFileRepo) Create(ctx context.Context, file *model.File) error {
	if m.createFn == nil {
		panic("mockFileRepo.Create not set")
	}
	return m.createFn(ctx, file)
}
func (m *mockFileRepo) FindByID(ctx context.Context, id int64) (*model.File, error) {
	if m.findByIDFn == nil {
		panic("mockFileRepo.FindByID not set")
	}
	return m.findByIDFn(ctx, id)
}
func (m *mockFileRepo) FindByIDAndOwner(ctx context.Context, id, ownerID int64) (*model.File, error) {
	if m.findByIDAndOwnerFn == nil {
		panic("mockFileRepo.FindByIDAndOwner not set")
	}
	return m.findByIDAndOwnerFn(ctx, id, ownerID)
}
func (m *mockFileRepo) FindByParentAndOwner(ctx context.Context, parentID, ownerID int64, includeDeleted bool) ([]model.File, error) {
	if m.findByParentAndOwnerFn == nil {
		panic("mockFileRepo.FindByParentAndOwner not set")
	}
	return m.findByParentAndOwnerFn(ctx, parentID, ownerID, includeDeleted)
}
func (m *mockFileRepo) FindByNameAndParent(ctx context.Context, ownerID, parentID int64, name string) (*model.File, error) {
	if m.findByNameAndParentFn == nil {
		panic("mockFileRepo.FindByNameAndParent not set")
	}
	return m.findByNameAndParentFn(ctx, ownerID, parentID, name)
}
func (m *mockFileRepo) ExistsByName(ctx context.Context, ownerID, parentID int64, name string) bool {
	if m.existsByNameFn == nil {
		panic("mockFileRepo.ExistsByName not set")
	}
	return m.existsByNameFn(ctx, ownerID, parentID, name)
}
func (m *mockFileRepo) Update(ctx context.Context, file *model.File) error {
	if m.updateFn == nil {
		panic("mockFileRepo.Update not set")
	}
	return m.updateFn(ctx, file)
}
func (m *mockFileRepo) SoftDelete(ctx context.Context, id int64) error {
	if m.softDeleteFn == nil {
		panic("mockFileRepo.SoftDelete not set")
	}
	return m.softDeleteFn(ctx, id)
}
func (m *mockFileRepo) Restore(ctx context.Context, id int64, newParentID *int64, newName string) error {
	if m.restoreFn == nil {
		panic("mockFileRepo.Restore not set")
	}
	return m.restoreFn(ctx, id, newParentID, newName)
}
func (m *mockFileRepo) FindTrash(ctx context.Context, ownerID int64) ([]model.File, error) {
	if m.findTrashFn == nil {
		panic("mockFileRepo.FindTrash not set")
	}
	return m.findTrashFn(ctx, ownerID)
}
func (m *mockFileRepo) Delete(ctx context.Context, id int64) error {
	if m.deleteFn == nil {
		panic("mockFileRepo.Delete not set")
	}
	return m.deleteFn(ctx, id)
}
func (m *mockFileRepo) FindAllDescendants(ctx context.Context, parentID int64) ([]model.File, error) {
	if m.findAllDescendantsFn == nil {
		panic("mockFileRepo.FindAllDescendants not set")
	}
	return m.findAllDescendantsFn(ctx, parentID)
}
func (m *mockFileRepo) BatchUpdateParent(ctx context.Context, fileIDs []int64, newParentID int64) error {
	if m.batchUpdateParentFn == nil {
		panic("mockFileRepo.BatchUpdateParent not set")
	}
	return m.batchUpdateParentFn(ctx, fileIDs, newParentID)
}
func (m *mockFileRepo) IsAncestor(ctx context.Context, fileID, targetID int64) (bool, error) {
	if m.isAncestorFn == nil {
		panic("mockFileRepo.IsAncestor not set")
	}
	return m.isAncestorFn(ctx, fileID, targetID)
}
func (m *mockFileRepo) Search(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error) {
	if m.searchFn == nil {
		panic("mockFileRepo.Search not set")
	}
	return m.searchFn(ctx, userID, keyword, folderID, sort)
}

type mockPhysicalFileRepo struct {
	createFn                        func(ctx context.Context, pf *model.PhysicalFile) error
	findByIDFn                      func(ctx context.Context, id int64) (*model.PhysicalFile, error)
	findByMD5Fn                     func(ctx context.Context, md5 string) (*model.PhysicalFile, error)
	incrementRefCountFn             func(ctx context.Context, id int64) error
	decrementRefCountFn             func(ctx context.Context, id int64, count int) (int, error)
	updateThumbnailFn               func(ctx context.Context, id int64, thumbnailPath string) error
	deleteFn                        func(ctx context.Context, id int64) error
	updateFn                        func(ctx context.Context, pf *model.PhysicalFile) error
	calculateUserStorageUsageFn     func(ctx context.Context, userID int64) (int64, error)
	calculateAllUserStorageUsageFn  func(ctx context.Context) (map[int64]int64, error)
}

func (m *mockPhysicalFileRepo) Create(ctx context.Context, pf *model.PhysicalFile) error {
	if m.createFn == nil {
		panic("mockPhysicalFileRepo.Create not set")
	}
	return m.createFn(ctx, pf)
}
func (m *mockPhysicalFileRepo) FindByID(ctx context.Context, id int64) (*model.PhysicalFile, error) {
	if m.findByIDFn == nil {
		panic("mockPhysicalFileRepo.FindByID not set")
	}
	return m.findByIDFn(ctx, id)
}
func (m *mockPhysicalFileRepo) FindByMD5(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
	if m.findByMD5Fn == nil {
		panic("mockPhysicalFileRepo.FindByMD5 not set")
	}
	return m.findByMD5Fn(ctx, md5)
}
func (m *mockPhysicalFileRepo) IncrementRefCount(ctx context.Context, id int64) error {
	if m.incrementRefCountFn == nil {
		panic("mockPhysicalFileRepo.IncrementRefCount not set")
	}
	return m.incrementRefCountFn(ctx, id)
}
func (m *mockPhysicalFileRepo) DecrementRefCount(ctx context.Context, id int64, count int) (int, error) {
	if m.decrementRefCountFn == nil {
		panic("mockPhysicalFileRepo.DecrementRefCount not set")
	}
	return m.decrementRefCountFn(ctx, id, count)
}
func (m *mockPhysicalFileRepo) UpdateThumbnail(ctx context.Context, id int64, thumbnailPath string) error {
	if m.updateThumbnailFn == nil {
		panic("mockPhysicalFileRepo.UpdateThumbnail not set")
	}
	return m.updateThumbnailFn(ctx, id, thumbnailPath)
}
func (m *mockPhysicalFileRepo) Delete(ctx context.Context, id int64) error {
	if m.deleteFn == nil {
		panic("mockPhysicalFileRepo.Delete not set")
	}
	return m.deleteFn(ctx, id)
}
func (m *mockPhysicalFileRepo) Update(ctx context.Context, pf *model.PhysicalFile) error {
	if m.updateFn == nil {
		panic("mockPhysicalFileRepo.Update not set")
	}
	return m.updateFn(ctx, pf)
}
func (m *mockPhysicalFileRepo) CalculateUserStorageUsage(ctx context.Context, userID int64) (int64, error) {
	if m.calculateUserStorageUsageFn == nil {
		return 0, nil
	}
	return m.calculateUserStorageUsageFn(ctx, userID)
}
func (m *mockPhysicalFileRepo) CalculateAllUserStorageUsage(ctx context.Context) (map[int64]int64, error) {
	if m.calculateAllUserStorageUsageFn == nil {
		return nil, nil
	}
	return m.calculateAllUserStorageUsageFn(ctx)
}

type mockUserRepo struct {
	findByIDFn       func(ctx context.Context, id int64) (*model.User, error)
	findByUsernameFn func(ctx context.Context, username string) (*model.User, error)
	createFn         func(ctx context.Context, user *model.User) error
	updatePasswordFn func(ctx context.Context, userID int64, passwordHash string) error
	getQuotaFn       func(ctx context.Context, userID int64) (*int64, error)
	setQuotaFn       func(ctx context.Context, userID int64, quota *int64) error
}

func (m *mockUserRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	if m.findByIDFn == nil {
		panic("mockUserRepo.FindByID not set")
	}
	return m.findByIDFn(ctx, id)
}
func (m *mockUserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	if m.findByUsernameFn == nil {
		panic("mockUserRepo.FindByUsername not set")
	}
	return m.findByUsernameFn(ctx, username)
}
func (m *mockUserRepo) Create(ctx context.Context, user *model.User) error {
	if m.createFn == nil {
		panic("mockUserRepo.Create not set")
	}
	return m.createFn(ctx, user)
}
func (m *mockUserRepo) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	if m.updatePasswordFn == nil {
		panic("mockUserRepo.UpdatePassword not set")
	}
	return m.updatePasswordFn(ctx, userID, passwordHash)
}
func (m *mockUserRepo) GetQuota(ctx context.Context, userID int64) (*int64, error) {
	if m.getQuotaFn == nil {
		return nil, nil
	}
	return m.getQuotaFn(ctx, userID)
}
func (m *mockUserRepo) SetQuota(ctx context.Context, userID int64, quota *int64) error {
	if m.setQuotaFn == nil {
		return nil
	}
	return m.setQuotaFn(ctx, userID, quota)
}

type mockStorage struct {
	generateFilePathFn func(physicalID int64, ext string) (relative, absolute string)
	toAbsPathFn        func(relative string) string
	thumbnailPathFn    func(physicalID int64) string
	tempChunkDirFn     func(uploadID string) string
	ensureParentDirFn  func(path string) error
}

func (m *mockStorage) GenerateFilePath(physicalID int64, ext string) (relative, absolute string) {
	if m.generateFilePathFn == nil {
		panic("mockStorage.GenerateFilePath not set")
	}
	return m.generateFilePathFn(physicalID, ext)
}
func (m *mockStorage) ToAbsPath(relative string) string {
	if m.toAbsPathFn == nil {
		panic("mockStorage.ToAbsPath not set")
	}
	return m.toAbsPathFn(relative)
}
func (m *mockStorage) ThumbnailPath(physicalID int64) string {
	if m.thumbnailPathFn == nil {
		panic("mockStorage.ThumbnailPath not set")
	}
	return m.thumbnailPathFn(physicalID)
}
func (m *mockStorage) TempChunkDir(uploadID string) string {
	if m.tempChunkDirFn == nil {
		panic("mockStorage.TempChunkDir not set")
	}
	return m.tempChunkDirFn(uploadID)
}
func (m *mockStorage) EnsureParentDir(path string) error {
	if m.ensureParentDirFn == nil {
		panic("mockStorage.EnsureParentDir not set")
	}
	return m.ensureParentDirFn(path)
}

type mockPasswordHasher struct {
	hashPasswordFn  func(password string) (string, error)
	checkPasswordFn func(password, hash string) bool
}

func (m *mockPasswordHasher) HashPassword(password string) (string, error) {
	if m.hashPasswordFn == nil {
		panic("mockPasswordHasher.HashPassword not set")
	}
	return m.hashPasswordFn(password)
}
func (m *mockPasswordHasher) CheckPassword(password, hash string) bool {
	if m.checkPasswordFn == nil {
		panic("mockPasswordHasher.CheckPassword not set")
	}
	return m.checkPasswordFn(password, hash)
}

type mockTokenGenerator struct {
	generateTokenFn func(userID int64, username, role string) (string, error)
}

func (m *mockTokenGenerator) GenerateToken(userID int64, username, role string) (string, error) {
	if m.generateTokenFn == nil {
		panic("mockTokenGenerator.GenerateToken not set")
	}
	return m.generateTokenFn(userID, username, role)
}

type mockShareRepo struct {
	createFn                 func(ctx context.Context, share *model.FileShare) error
	findByTokenFn            func(ctx context.Context, token string) (*model.FileShare, error)
	findByOwnerFn            func(ctx context.Context, ownerID int64) ([]model.FileShare, error)
	findByFileFn             func(ctx context.Context, fileID int64) ([]model.FileShare, error)
	revokeFn                 func(ctx context.Context, id, ownerID int64) error
	incrementDownloadCountFn func(ctx context.Context, token string) (bool, error)
}

func (m *mockShareRepo) Create(ctx context.Context, share *model.FileShare) error {
	if m.createFn == nil {
		panic("mockShareRepo.Create not set")
	}
	return m.createFn(ctx, share)
}
func (m *mockShareRepo) FindByToken(ctx context.Context, token string) (*model.FileShare, error) {
	if m.findByTokenFn == nil {
		panic("mockShareRepo.FindByToken not set")
	}
	return m.findByTokenFn(ctx, token)
}
func (m *mockShareRepo) FindByOwner(ctx context.Context, ownerID int64) ([]model.FileShare, error) {
	if m.findByOwnerFn == nil {
		panic("mockShareRepo.FindByOwner not set")
	}
	return m.findByOwnerFn(ctx, ownerID)
}
func (m *mockShareRepo) FindByFile(ctx context.Context, fileID int64) ([]model.FileShare, error) {
	if m.findByFileFn == nil {
		panic("mockShareRepo.FindByFile not set")
	}
	return m.findByFileFn(ctx, fileID)
}
func (m *mockShareRepo) Revoke(ctx context.Context, id, ownerID int64) error {
	if m.revokeFn == nil {
		panic("mockShareRepo.Revoke not set")
	}
	return m.revokeFn(ctx, id, ownerID)
}
func (m *mockShareRepo) IncrementDownloadCount(ctx context.Context, token string) (bool, error) {
	if m.incrementDownloadCountFn == nil {
		panic("mockShareRepo.IncrementDownloadCount not set")
	}
	return m.incrementDownloadCountFn(ctx, token)
}

type mockImageProcessor struct {
	processImageFn func(ctx context.Context, physicalID int64) error
}

func (m *mockImageProcessor) ProcessImage(ctx context.Context, physicalID int64) error {
	if m.processImageFn == nil {
		panic("mockImageProcessor.ProcessImage not set")
	}
	return m.processImageFn(ctx, physicalID)
}
