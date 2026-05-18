// internal/repository/file.go
package repository

import (
	"cloudbox/internal/model"
	"context"
	"fmt"
	"strings"

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

func (r *FileRepository) FindByParentAndOwner(ctx context.Context, parentID, ownerID int64, includeDeleted bool) ([]model.File, error) {
	query := r.db.WithContext(ctx).
		Where("owner_id = ?", ownerID)

	if parentID == 0 {
		query = query.Where("parent_id = 0 OR parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", parentID)
	}

	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}

	var files []model.File
	err := query.Order("is_folder DESC, name ASC").Find(&files).Error
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
		Update("deleted_at", gorm.Expr("datetime('now')")).Error
}

func (r *FileRepository) Restore(ctx context.Context, id int64, newParentID *int64, newName string) error {
	updates := map[string]interface{}{
		"deleted_at": nil,
		"updated_at": gorm.Expr("datetime('now')"),
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

func NullInt64(v int64) model.NullInt64 {
	return model.NullInt64{Int64: v, Valid: true}
}

func NullInt64Ptr(v *int64) model.NullInt64 {
	if v == nil {
		return model.NullInt64{}
	}
	return model.NullInt64{Int64: *v, Valid: true}
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