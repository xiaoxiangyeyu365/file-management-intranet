// internal/repository/file.go
package repository

import (
	"cloudbox/internal/model"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type FileRepository struct {
	db *gorm.DB
}

func NewFileRepository(db *gorm.DB) *FileRepository {
	return &FileRepository{db: db}
}

func (r *FileRepository) Create(ctx context.Context, file *model.File) error {
	return r.db.WithContext(ctx).Create(file).Error
}

func (r *FileRepository) FindByID(ctx context.Context, id int64) (*model.File, error) {
	var file model.File
	err := r.db.WithContext(ctx).First(&file, id).Error
	if err != nil {
		return nil, err
	}
	if file.ContentRef != 0 {
		r.db.WithContext(ctx).First(&file.Physical, file.ContentRef)
	}
	return &file, nil
}

func (r *FileRepository) FindByIDAndOwner(ctx context.Context, id, ownerID int64) (*model.File, error) {
	var file model.File
	err := r.db.WithContext(ctx).
		Where("id = ? AND owner_id = ?", id, ownerID).
		First(&file).Error
	if err != nil {
		return nil, err
	}
	if file.ContentRef != 0 {
		r.db.WithContext(ctx).First(&file.Physical, file.ContentRef)
	}
	return &file, nil
}

func (r *FileRepository) FindByParentAndOwner(ctx context.Context, parentID, ownerID int64, includeDeleted bool, sortBy, sortOrder string) ([]model.File, error) {
	var files []model.File
	if err := applyListSort(r.parentQuery(ctx, parentID, ownerID, includeDeleted), sortBy, sortOrder).Find(&files).Error; err != nil {
		return nil, err
	}

	r.loadPhysicalFiles(ctx, files)
	return files, nil
}

func (r *FileRepository) FindByParentAndOwnerPaged(ctx context.Context, parentID, ownerID int64, page, pageSize int, sortBy, sortOrder string) ([]model.File, int64, error) {
	base := r.parentQuery(ctx, parentID, ownerID, false)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var files []model.File
	if err := applyListSort(base, sortBy, sortOrder).Offset(offset).Limit(pageSize).Find(&files).Error; err != nil {
		return nil, 0, err
	}

	r.loadPhysicalFiles(ctx, files)
	return files, total, nil
}

func (r *FileRepository) parentQuery(ctx context.Context, parentID, ownerID int64, includeDeleted bool) *gorm.DB {
	query := r.db.WithContext(ctx).Model(&model.File{}).Where("owner_id = ?", ownerID)

	if parentID == 0 {
		query = query.Where("parent_id = 0 OR parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", parentID)
	}

	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	return query
}

// applyListSort orders the listing. Folders always come first, and the
// remaining files are ordered by the requested field. sortBy/sortOrder are
// whitelisted here as a final line of defense against SQL injection.
func applyListSort(query *gorm.DB, sortBy, sortOrder string) *gorm.DB {
	if sortBy == "size" {
		// files.physical_ref may be 0 for folders or legacy rows, hence LEFT JOIN.
		query = query.
			Joins("LEFT JOIN physical_files pf ON pf.id = files.physical_ref").
			Select("files.*")
	}

	dir := "ASC"
	if sortOrder == "desc" {
		dir = "DESC"
	}

	var order string
	switch sortBy {
	case "size":
		order = fmt.Sprintf("is_folder DESC, COALESCE(pf.size, 0) %s, name ASC", dir)
	case "updatedAt":
		order = fmt.Sprintf("is_folder DESC, updated_at %s, name ASC", dir)
	default:
		order = fmt.Sprintf("is_folder DESC, name %s", dir)
	}
	return query.Order(order)
}

func (r *FileRepository) loadPhysicalFiles(ctx context.Context, files []model.File) {
	for i := range files {
		if files[i].ContentRef != 0 {
			r.db.WithContext(ctx).First(&files[i].Physical, files[i].ContentRef)
		}
	}
}

func (r *FileRepository) FindByNameAndParent(ctx context.Context, ownerID, parentID int64, name string) (*model.File, error) {
	query := r.db.WithContext(ctx).
		Where("owner_id = ? AND name = ? AND deleted_at IS NULL", ownerID, name)

	if parentID == 0 {
		query = query.Where("parent_id = 0 OR parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", parentID)
	}

	var file model.File
	err := query.First(&file).Error
	if err != nil {
		return nil, err
	}
	if file.ContentRef != 0 {
		r.db.WithContext(ctx).First(&file.Physical, file.ContentRef)
	}
	return &file, nil
}

func (r *FileRepository) ExistsByName(ctx context.Context, ownerID, parentID int64, name string) bool {
	query := r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("owner_id = ? AND name = ? AND deleted_at IS NULL", ownerID, name)

	if parentID == 0 {
		query = query.Where("parent_id = 0 OR parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", parentID)
	}

	var count int64
	query.Count(&count)
	return count > 0
}

func (r *FileRepository) Update(ctx context.Context, file *model.File) error {
	return r.db.WithContext(ctx).Save(file).Error
}

func (r *FileRepository) SoftDelete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

func (r *FileRepository) Restore(ctx context.Context, id int64, newParentID *int64, newName string) error {
	updates := map[string]interface{}{
		"deleted_at": nil,
		"updated_at": time.Now(),
	}
	if newParentID != nil {
		updates["parent_id"] = *newParentID
	}
	if newName != "" {
		updates["name"] = newName
	}

	return r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *FileRepository) FindTrash(ctx context.Context, ownerID int64) ([]model.File, error) {
	var files []model.File
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND deleted_at IS NOT NULL", ownerID).
		Order("deleted_at DESC").
		Find(&files).Error
	if err != nil {
		return nil, err
	}

	// Manually load Physical for files that have one
	for i := range files {
		if files[i].ContentRef != 0 {
			r.db.WithContext(ctx).First(&files[i].Physical, files[i].ContentRef)
		}
	}
	return files, nil
}

func (r *FileRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.File{}, id).Error
}

func (r *FileRepository) FindAllDescendants(ctx context.Context, parentID int64) ([]model.File, error) {
	var files []model.File

	query := `
		WITH RECURSIVE descendants AS (
			SELECT * FROM files WHERE parent_id = ?
			UNION ALL
			SELECT f.* FROM files f
			INNER JOIN descendants d ON f.parent_id = d.id
		)
		SELECT * FROM descendants
	`

	err := r.db.WithContext(ctx).Raw(query, parentID).Scan(&files).Error
	if err != nil {
		return nil, err
	}

	// Manually load Physical for files that have one
	for i := range files {
		if files[i].ContentRef != 0 {
			r.db.WithContext(ctx).First(&files[i].Physical, files[i].ContentRef)
		}
	}
	return files, nil
}

// FindAllByOwner returns all non-deleted files for a user with Physical preloaded.
func (r *FileRepository) FindAllByOwner(ctx context.Context, ownerID int64) ([]model.File, error) {
	var files []model.File
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND deleted_at IS NULL", ownerID).
		Find(&files).Error
	if err != nil {
		return nil, err
	}

	for i := range files {
		if files[i].ContentRef != 0 {
			r.db.WithContext(ctx).First(&files[i].Physical, files[i].ContentRef)
		}
	}
	return files, nil
}

func (r *FileRepository) BatchUpdateParent(ctx context.Context, fileIDs []int64, newParentID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.File{}).
		Where("id IN ?", fileIDs).
		Update("parent_id", newParentID).Error
}

func (r *FileRepository) IsAncestor(ctx context.Context, fileID, targetID int64) (bool, error) {
	query := `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id FROM files WHERE id = ?
			UNION ALL
			SELECT f.id, f.parent_id FROM files f
			INNER JOIN ancestors a ON f.id = a.parent_id
			WHERE f.parent_id IS NOT NULL
		)
		SELECT COUNT(*) FROM ancestors WHERE id = ?
	`

	var count int64
	err := r.db.WithContext(ctx).Raw(query, fileID, targetID).Scan(&count).Error
	return count > 0, err
}

func NullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}

func NullInt64Ptr(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func (r *FileRepository) Search(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error) {
	var files []model.File

	if folderID == nil {
		query := r.db.WithContext(ctx).
			Where("owner_id = ? AND deleted_at IS NULL AND name LIKE ?", userID, "%"+keyword+"%")
		query = r.applySearchSort(query, sort, keyword)
		err := query.Find(&files).Error
		if err != nil {
			return nil, err
		}
		for i := range files {
			if files[i].ContentRef != 0 {
				r.db.WithContext(ctx).First(&files[i].Physical, files[i].ContentRef)
			}
		}
		return files, nil
	}

	query := `
		WITH RECURSIVE subfolders AS (
			SELECT id FROM files
			WHERE id = ? AND owner_id = ? AND is_folder = 1 AND deleted_at IS NULL
			UNION ALL
			SELECT f.id FROM files f
			JOIN subfolders s ON f.parent_id = s.id
			WHERE f.owner_id = ? AND f.is_folder = 1 AND f.deleted_at IS NULL
		)
		SELECT f.* FROM files f
		JOIN subfolders s ON f.parent_id = s.id
		WHERE f.owner_id = ?
		  AND f.deleted_at IS NULL
		  AND f.name LIKE ?
	`

	orderBy := r.buildSearchOrderBy(sort, keyword)
	query += orderBy

	err := r.db.WithContext(ctx).Raw(query, *folderID, userID, userID, userID, "%"+keyword+"%").Scan(&files).Error
	if err != nil {
		return nil, err
	}

	for i := range files {
		if files[i].ContentRef != 0 {
			r.db.WithContext(ctx).First(&files[i].Physical, files[i].ContentRef)
		}
	}
	return files, nil
}

func (r *FileRepository) applySearchSort(query *gorm.DB, sort, keyword string) *gorm.DB {
	switch sort {
	case "time":
		return query.Order("updated_at DESC")
	case "name":
		return query.Order("name ASC")
	default:
		escapedKeyword := sanitizeSearchKeyword(keyword)
		return query.Order(fmt.Sprintf(
			"CASE WHEN name = '%s' THEN 0 WHEN name LIKE '%s%%' THEN 1 ELSE 2 END, name ASC",
			escapedKeyword, escapedKeyword))
	}
}

func (r *FileRepository) buildSearchOrderBy(sort, keyword string) string {
	switch sort {
	case "time":
		return " ORDER BY updated_at DESC"
	case "name":
		return " ORDER BY name ASC"
	default:
		escapedKeyword := sanitizeSearchKeyword(keyword)
		return fmt.Sprintf(" ORDER BY CASE WHEN name = '%s' THEN 0 WHEN name LIKE '%s%%' THEN 1 ELSE 2 END, name ASC",
			escapedKeyword, escapedKeyword)
	}
}

func sanitizeSearchKeyword(keyword string) string {
	result := strings.ReplaceAll(keyword, "\\", "\\\\")
	result = strings.ReplaceAll(result, "'", "''")
	return result
}