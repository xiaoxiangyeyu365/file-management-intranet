// internal/config/config.go
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Upload   UploadConfig   `yaml:"upload"`
	JWT      JWTConfig      `yaml:"jwt"`
	Log      LogConfig      `yaml:"log"`
	Admin    AdminConfig    `yaml:"admin"`
	Auth     AuthConfig     `yaml:"auth"`
	Share    ShareConfig    `yaml:"share"`
	Disk     DiskConfig     `yaml:"disk"`
	Audit    AuditConfig    `yaml:"audit"`
	TLS      TLSConfig      `yaml:"tls"`
	AI       AIConfig       `yaml:"ai"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type"` // "sqlite" or "mysql"
	Path     string `yaml:"path"` // For SQLite only
	Host     string `yaml:"host"` // For MySQL
	Port     int    `yaml:"port"` // For MySQL
	Username string `yaml:"username"` // For MySQL
	Password string `yaml:"password"` // For MySQL
	Name     string `yaml:"name"` // For MySQL (database name)
}

type StorageConfig struct {
	Root       string `yaml:"root"`
	Temp       string `yaml:"temp"`
	Thumbnails string `yaml:"thumbnails"`
}

type UploadConfig struct {
	ChunkSize   int64         `yaml:"chunk_size"`
	MaxConcurrent int         `yaml:"max_concurrent"`
	TempExpire  time.Duration `yaml:"temp_expire"`
}

type JWTConfig struct {
	Secret string        `yaml:"secret"`
	Expire time.Duration `yaml:"expire"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type AuthConfig struct {
	Registration     bool `yaml:"registration"`
	ApprovalRequired bool `yaml:"approval_required"`
}

type ShareConfig struct {
	Secret        string `yaml:"secret"`
	TokenLength   int    `yaml:"token_length"`
	CredentialTTL int    `yaml:"credential_ttl"`
}

type DiskConfig struct {
	DefaultQuota int64 `yaml:"default_quota"`
}

type AuditConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

type TLSConfig struct {
	Enabled bool     `yaml:"enabled"`
	Port    int      `yaml:"port"`
	Cert    string   `yaml:"cert"`
	Key     string   `yaml:"key"`
	CACert  string   `yaml:"ca_cert"`
	Hosts   []string `yaml:"hosts"`
}

type AIConfig struct {
	Enabled          bool   `yaml:"enabled"`
	BaseURL          string `yaml:"base_url"`
	APIKey           string `yaml:"api_key"`
	Model            string `yaml:"model"`
	VisionModel      string `yaml:"vision_model"`
	MaxContentLength int    `yaml:"max_content_length"`
	MaxConcurrent    int    `yaml:"max_concurrent"`
	AutoDocument     bool   `yaml:"auto_document"`
	AutoImage        bool   `yaml:"auto_image"`
	AutoVideo        bool   `yaml:"auto_video"`
	Timeout          int    `yaml:"timeout"`
	SummaryPrompt    string `yaml:"summary_prompt"`
}

var (
	cfg  *Config
	once sync.Once
)

func Load() *Config {
	once.Do(func() {
		cfg = &Config{
			Server: ServerConfig{
				Host: "0.0.0.0",
				Port: 8080,
			},
			Database: DatabaseConfig{
				Type:     "mysql",
				Host:     "localhost",
				Port:     3306,
				Username: "root",
				Password: "",
				Name:     "cloudbox",
			},
			Storage: StorageConfig{
				Root:       "./data/files",
				Temp:       "./data/temp",
				Thumbnails: "./data/thumbnails",
			},
			Upload: UploadConfig{
				ChunkSize:     5 * 1024 * 1024,
				MaxConcurrent: 3,
				TempExpire:    24 * time.Hour,
			},
			JWT: JWTConfig{
				Expire: 24 * time.Hour,
			},
			Log: LogConfig{
				Level: "info",
				File:  "./data/logs/cloudbox.log",
			},
			Admin: AdminConfig{
				Username: "admin",
				Password: "admin123",
			},
			Auth: AuthConfig{
				Registration:     true,
				ApprovalRequired: true,
			},
			Share: ShareConfig{
				TokenLength:   8,
				CredentialTTL: 300,
			},
			Disk: DiskConfig{
				DefaultQuota: 10 * 1024 * 1024 * 1024, // 10GB
			},
			Audit: AuditConfig{
				RetentionDays: 90,
			},
			TLS: TLSConfig{
				Enabled: false,
				Port:    8443,
				Cert:    "./data/tls/server.crt",
				Key:     "./data/tls/server.key",
				CACert:  "./data/tls/ca.crt",
			},
			AI: AIConfig{
				Enabled:          false,
				BaseURL:          "https://api.deepseek.com/v1",
				Model:            "deepseek-chat",
				VisionModel:      "",
				MaxContentLength: 50000,
				MaxConcurrent:    2,
				AutoDocument:     true,
				AutoImage:        false,
				AutoVideo:        false,
				Timeout:          30,
				SummaryPrompt:    "请用中文为以下文档内容生成一段简洁摘要（不超过200字）和5个关键标签。格式：\n摘要：...\n标签：标签1,标签2,标签3,标签4,标签5",
			},
		}

		// Find config file
		configPaths := []string{"./config.yaml", "./configs/config.yaml"}
		var configFile string
		for _, p := range configPaths {
			if _, err := os.Stat(p); err == nil {
				configFile = p
				break
			}
		}

		if configFile != "" {
			data, err := os.ReadFile(configFile)
			if err != nil {
				log.Fatalf("failed to read config: %v", err)
			}
			if err := yaml.Unmarshal(data, cfg); err != nil {
				log.Fatalf("failed to parse config: %v", err)
			}
		}

		// JWT secret from env
		if cfg.JWT.Secret == "" {
			cfg.JWT.Secret = os.Getenv("JWT_SECRET")
		}
		if cfg.JWT.Secret == "" {
			log.Println("WARNING: JWT_SECRET not set, using random key (not suitable for production)")
			cfg.JWT.Secret = generateRandomKey()
		}

		if cfg.Share.Secret == "" {
			cfg.Share.Secret = generateRandomSecret()
			log.Println("warning: share.secret not configured, using random secret (shares will break on restart)")
		}

		if cfg.AI.APIKey == "" {
			cfg.AI.APIKey = os.Getenv("AI_API_KEY")
		}

		// Get base directory: executable directory or current working directory
		exePath, err := os.Executable()
		exeDir := ""
		if err == nil {
			exeDir = filepath.Dir(exePath)
		}
		// For "go run", executable is in temp dir, use cwd instead
		if strings.Contains(exeDir, "go-build") || exeDir == "" {
			exeDir, _ = os.Getwd()
		}

		cfg.Storage.Root = toAbsPath(exeDir, cfg.Storage.Root)
		cfg.Storage.Temp = toAbsPath(exeDir, cfg.Storage.Temp)
		cfg.Storage.Thumbnails = toAbsPath(exeDir, cfg.Storage.Thumbnails)
		cfg.Log.File = toAbsPath(exeDir, cfg.Log.File)
		cfg.TLS.Cert = toAbsPath(exeDir, cfg.TLS.Cert)
		cfg.TLS.Key = toAbsPath(exeDir, cfg.TLS.Key)
		cfg.TLS.CACert = toAbsPath(exeDir, cfg.TLS.CACert)

		// Create directories
		mkdirAll(cfg.Storage.Root)
		mkdirAll(cfg.Storage.Temp)
		mkdirAll(cfg.Storage.Thumbnails)
		mkdirAll(filepath.Dir(cfg.Database.Path))
		mkdirAll(filepath.Dir(cfg.Log.File))
		mkdirAll(filepath.Dir(cfg.TLS.Cert))

		if cfg.TLS.Enabled && cfg.TLS.Port == cfg.Server.Port {
			log.Fatalf("TLS port (%d) must differ from HTTP port (%d)", cfg.TLS.Port, cfg.Server.Port)
		}
	})
	return cfg
}

func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

func toAbsPath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

func mkdirAll(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		log.Fatalf("failed to create directory %s: %v", path, err)
	}
}

func generateRandomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("failed to generate random key: %v", err)
	}
	return hex.EncodeToString(b)
}

func generateRandomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
