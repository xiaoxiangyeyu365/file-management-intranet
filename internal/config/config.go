// internal/config/config.go
package config

import (
	"crypto/rand"
	"encoding/hex"
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

		// Create directories
		mkdirAll(cfg.Storage.Root)
		mkdirAll(cfg.Storage.Temp)
		mkdirAll(cfg.Storage.Thumbnails)
		mkdirAll(filepath.Dir(cfg.Database.Path))
		mkdirAll(filepath.Dir(cfg.Log.File))
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
