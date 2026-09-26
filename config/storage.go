package config

import (
	"github.com/spf13/viper"
)

type StorageConfig struct {
	Driver        string
	BasePath      string
	BasePathTrash string
	Bucket        string
	Region        string
	AccessKey     string
	SecretKey     string
}

func NewStorageConfig(viperConfig *viper.Viper) StorageConfig {
	return StorageConfig{
		Driver:        viperConfig.GetString("FILESYSTEM_DRIVER"),
		BasePath:      viperConfig.GetString("LOCAL_STORAGE_PATH"),
		BasePathTrash: viperConfig.GetString("LOCAL_STORAGE_PATH_TRASH"),
		Bucket:        viperConfig.GetString("AWS_BUCKET"),
		Region:        viperConfig.GetString("AWS_REGION"),
		AccessKey:     viperConfig.GetString("AWS_ACCESS_KEY_ID"),
		SecretKey:     viperConfig.GetString("AWS_SECRET_ACCESS_KEY"),
	}
}
