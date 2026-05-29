package repository

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"fmt"
	"log"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) *gorm.DB {
	var err error

	if cfg.Database.Type == "mysql" {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.Username,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Name,
		)
		DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	} else {
		DB, err = gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	}

	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Migrate all models
	if err := DB.AutoMigrate(
		&model.User{},
		&model.PhysicalFile{},
	); err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	// Ensure existing users have status and password_changed set
	DB.Exec("UPDATE users SET status = 'approved' WHERE status IS NULL OR status = ''")
	DB.Exec("UPDATE users SET password_changed = 1 WHERE password_changed IS NULL")

	// Create files table (GORM doesn't handle it well with custom fields)
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

	// Create file_shares table
	if err := createFileSharesTable(); err != nil {
		log.Fatalf("failed to create file_shares table: %v", err)
	}

	// Create default admin
	createDefaultAdmin(cfg)

	return DB
}

func createFilesTable() error {
	// Check if table exists
	var count int64
	if DB.Dialector.Name() == "sqlite" {
		DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='files'").Scan(&count)
	} else {
		DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'cloudbox' AND table_name = 'files'").Scan(&count)
	}
	if count > 0 {
		return nil
	}

	if DB.Dialector.Name() == "sqlite" {
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

	// MySQL
	sql := `CREATE TABLE files (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		parent_id BIGINT,
		owner_id BIGINT NOT NULL,
		is_folder BOOLEAN DEFAULT FALSE,
		deleted_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		physical_ref BIGINT
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	return DB.Exec(sql).Error
}

func createIndexes() error {
	indexes := []string{
		"CREATE INDEX idx_files_owner_parent_deleted ON files(owner_id, parent_id, deleted_at)",
		"CREATE INDEX idx_files_owner_deleted ON files(owner_id, deleted_at)",
		"CREATE INDEX idx_files_parent_name_deleted ON files(parent_id, name, deleted_at)",
		"CREATE INDEX idx_files_physical_ref ON files(physical_ref)",
		"CREATE UNIQUE INDEX idx_physical_files_md5 ON physical_files(md5)",
	}

	for _, idx := range indexes {
		// Ignore "index already exists" errors
		DB.Exec(idx)
	}
	return nil
}

func createClipboardTable() error {
	// Check if table exists
	var count int64
	if DB.Dialector.Name() == "sqlite" {
		DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='clipboard_records'").Scan(&count)
	} else {
		DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'cloudbox' AND table_name = 'clipboard_records'").Scan(&count)
	}
	if count > 0 {
		return nil
	}

	if DB.Dialector.Name() == "sqlite" {
		sql := `CREATE TABLE clipboard_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			device_name TEXT DEFAULT '未命名设备',
			user_id INTEGER NOT NULL,
			pinned BOOLEAN DEFAULT FALSE,
			created_at DATETIME NOT NULL
		)`
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
	} else {
		sql := `CREATE TABLE clipboard_records (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			content TEXT NOT NULL,
			device_name VARCHAR(255) DEFAULT '未命名设备',
			user_id BIGINT NOT NULL,
			pinned BOOLEAN DEFAULT FALSE,
			created_at DATETIME NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
	}

	// Create index for MySQL
	if DB.Dialector.Name() == "mysql" {
		DB.Exec("CREATE INDEX idx_clipboard_user_pinned_created ON clipboard_records(user_id, pinned, created_at)")
	}

	return nil
}

func createFileSharesTable() error {
	// Check if table exists
	var count int64
	if DB.Dialector.Name() == "sqlite" {
		DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='file_shares'").Scan(&count)
	} else {
		DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'cloudbox' AND table_name = 'file_shares'").Scan(&count)
	}
	if count > 0 {
		return nil
	}

	if DB.Dialector.Name() == "sqlite" {
		sql := `CREATE TABLE file_shares (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token VARCHAR(8) NOT NULL UNIQUE,
			file_id INTEGER NOT NULL,
			owner_id INTEGER NOT NULL,
			password_hash VARCHAR(255),
			expires_at DATETIME,
			max_downloads INTEGER DEFAULT 0,
			download_count INTEGER DEFAULT 0,
			revoked BOOLEAN DEFAULT 0,
			created_at DATETIME NOT NULL
		)`
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
		// Create indexes separately for SQLite
		DB.Exec("CREATE INDEX idx_file_shares_file_id ON file_shares(file_id)")
		DB.Exec("CREATE INDEX idx_file_shares_owner_id ON file_shares(owner_id)")
	} else {
		sql := `CREATE TABLE file_shares (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			token VARCHAR(8) NOT NULL UNIQUE,
			file_id BIGINT NOT NULL,
			owner_id BIGINT NOT NULL,
			password_hash VARCHAR(255),
			expires_at DATETIME,
			max_downloads INT DEFAULT 0,
			download_count INT DEFAULT 0,
			revoked BOOLEAN DEFAULT FALSE,
			created_at DATETIME NOT NULL,
			INDEX idx_file_shares_file_id (file_id),
			INDEX idx_file_shares_owner_id (owner_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
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