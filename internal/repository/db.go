// internal/repository/db.go
package repository

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"log"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) *gorm.DB {
	var err error

	DB, err = gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Auto migrate
	if err := DB.AutoMigrate(&model.User{}, &model.PhysicalFile{}, &model.File{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	// Create indexes
	if err := createIndexes(); err != nil {
		log.Fatalf("failed to create indexes: %v", err)
	}

	// Create default admin
	createDefaultAdmin(cfg)

	return DB
}

func createIndexes() error {
	// Files indexes
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_owner_parent_deleted ON files(owner_id, parent_id, deleted_at)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_owner_deleted ON files(owner_id, deleted_at)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_parent_name_deleted ON files(parent_id, name, deleted_at)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_physical_id ON files(physical_id)").Error; err != nil {
		return err
	}

	// Physical files index
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_physical_files_md5 ON physical_files(md5)").Error; err != nil {
		return err
	}

	return nil
}

func createDefaultAdmin(cfg *config.Config) {
	var count int64
	DB.Model(&model.User{}).Count(&count)
	if count > 0 {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash admin password: %v", err)
	}

	admin := &model.User{
		Username:     cfg.Admin.Username,
		PasswordHash: string(hash),
		Role:         "admin",
	}

	if err := DB.Create(admin).Error; err != nil {
		log.Fatalf("failed to create admin user: %v", err)
	}

	log.Printf("Created default admin user: %s", cfg.Admin.Username)
}
