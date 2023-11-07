package datasafed

import (
	"fmt"

	"github.com/wal-g/wal-g/pkg/storages/storage"
)

var defaultConfigFilePath = "/etc/datasafed/datasafed.conf"

// TODO: Unit tests
func ConfigureStorage(_ string, _ map[string]string, rootWraps ...storage.WrapRootFolder) (storage.HashableStorage, error) {
	config := &Config{
		ConfigFilePath: defaultConfigFilePath,
	}

	st, err := NewStorage(config, rootWraps...)
	if err != nil {
		return nil, fmt.Errorf("create datasafed storage: %w", err)
	}
	return st, nil
}
