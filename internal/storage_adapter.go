package internal

import (
	"github.com/spf13/viper"

	conf "github.com/wal-g/wal-g/internal/config"
	"github.com/wal-g/wal-g/pkg/storages/azure"
	"github.com/wal-g/wal-g/pkg/storages/datasafed"
	"github.com/wal-g/wal-g/pkg/storages/fs"
	"github.com/wal-g/wal-g/pkg/storages/gcs"
	"github.com/wal-g/wal-g/pkg/storages/s3"
	"github.com/wal-g/wal-g/pkg/storages/sh"
	"github.com/wal-g/wal-g/pkg/storages/storage"
	"github.com/wal-g/wal-g/pkg/storages/swift"
)

type StorageAdapter struct {
	storageType  string
	noPrefix     bool
	settingNames []string
	configure    ConfigureStorageFunc
}

type ConfigureStorageFunc func(
	prefix string,
	settings map[string]string,
	rootWraps ...storage.WrapRootFolder,
) (storage.HashableStorage, error)

func (adapter *StorageAdapter) PrefixSettingKey() string {
	if adapter.noPrefix {
		return adapter.storageType
	}
	return adapter.storageType + "_PREFIX"
}

func (adapter *StorageAdapter) loadSettings(config *viper.Viper) map[string]string {
	settings := make(map[string]string)

	for _, settingName := range adapter.settingNames {
		settingValue := config.GetString(settingName)
		if config.IsSet(settingName) {
			settings[settingName] = settingValue
			/* prefer config values */
			continue
		}

		settingValue, ok := conf.GetWaleCompatibleSettingFrom(settingName, config)
		if !ok {
			settingValue, ok = conf.GetSetting(settingName)
		}
		if ok {
			settings[settingName] = settingValue
		}
	}
	return settings
}

var StorageAdapters = []StorageAdapter{
	{"S3", false, s3.SettingList, s3.ConfigureStorage},
	{"FILE", false, nil, fs.ConfigureStorage},
	{"GS", false, gcs.SettingList, gcs.ConfigureStorage},
	{"AZ", false, azure.SettingList, azure.ConfigureStorage},
	{"SWIFT", false, swift.SettingList, swift.ConfigureStorage},
	{"SSH", false, sh.SettingList, sh.ConfigureStorage},
	{"DATASAFED_CONFIG", true, sh.SettingList, datasafed.ConfigureStorage},
}
