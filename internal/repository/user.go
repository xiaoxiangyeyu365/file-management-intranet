// internal/repository/user.go
package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Update("password_hash", passwordHash).Error
}

func (r *UserRepository) FindAll(ctx context.Context, status string) ([]model.User, error) {
	var users []model.User
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Find(&users).Error
	return users, err
}

func (r *UserRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *UserRepository) UpdateStatus(ctx context.Context, userID int64, status string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("status", status).Error
}

func (r *UserRepository) UpdateRole(ctx context.Context, userID int64, role string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("role", role).Error
}

func (r *UserRepository) UpdatePasswordChanged(ctx context.Context, userID int64, changed bool) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password_changed", changed).Error
}

func (r *UserRepository) DeleteByID(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, userID).Error
}

func (r *UserRepository) GetQuota(ctx context.Context, userID int64) (*int64, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Select("disk_quota").First(&user, userID).Error; err != nil {
		return nil, err
	}
	return user.DiskQuota, nil
}

func (r *UserRepository) SetQuota(ctx context.Context, userID int64, quota *int64) error {
	return r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Update("disk_quota", quota).Error
}
