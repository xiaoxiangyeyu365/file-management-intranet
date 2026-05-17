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

	// Migrate User and PhysicalFile only
	if err := DB.AutoMigrate(&model.User{}, &model.PhysicalFile{}); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	// Create files table manually to avoid GORM relation issues
	if err := createFilesTable(); err != nil {
		log.Fatalf("failed to create files table: %v", err)
	}

	// Create indexes
	if err := createIndexes(); err != nil {
		log.Fatalf("failed to create indexes: %v", err)
	}

	// Create clipboard table
	if err := createClipboardTable(); err != nil {
		log.Fatalf("failed to create clipboard table: %v", err)
	}
	if err := createClipboardIndexes(); err != nil {
		log.Fatalf("failed to create clipboard indexes: %v", err)
	}

	// Create default admin
	createDefaultAdmin(cfg)

	return DB
}

func createFilesTable() error {
	// Check if table exists
	var count int64
	DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='files'").Scan(&count)
	if count > 0 {
		return nil
	}

	sql := `CREATE TABLE files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		parent_id INTEGER,
		owner_id INTEGER NOT NULL,
		is_folder BOOLEAN DEFAULT FALSE,
		deleted_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		physical_ref INTEGER
	)`
	return DB.Exec(sql).Error
}

func createIndexes() error {
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_owner_parent_deleted ON files(owner_id, parent_id, deleted_at)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_owner_deleted ON files(owner_id, deleted_at)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_parent_name_deleted ON files(parent_id, name, deleted_at)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_physical_ref ON files(physical_ref)").Error; err != nil {
		return err
	}
	if err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_physical_files_md5 ON physical_files(md5)").Error; err != nil {
		return err
	}
	return nil
}

func createClipboardTable() error {
	var count int64
	DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='clipboard_records'").Scan(&count)
	if count > 0 {
		return nil
	}

	sql := `CREATE TABLE clipboard_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content TEXT NOT NULL,
		device_name TEXT DEFAULT '未命名设备',
		user_id INTEGER NOT NULL,
		pinned BOOLEAN DEFAULT FALSE,
		created_at DATETIME NOT NULL
	)`
	return DB.Exec(sql).Error
}

func createClipboardIndexes() error {
	if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_clipboard_user_pinned_created ON clipboard_records(user_id, pinned, created_at)").Error; err != nil {
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
