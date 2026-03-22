package serverconfig

import (
	"fmt"
	"io/fs"
	"os"
)

type sanatize func(data []byte) ([]byte, error)

var sanatizers map[string]sanatize

func init() {
	sanatizers = map[string]sanatize{
		"server.properties":       sanatizeServerProperties,
		"config/paper-global.yml": sanatizePaperGlobal,
	}
}

func SanitizeConfigs(root *os.Root) error {
	if err := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		sanatize, ok := sanatizers[path]
		if !ok {
			return nil
		}

		data, err := root.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		sanatized, err := sanatize(data)
		if err != nil {
			return fmt.Errorf("sanatize file %s: %w", path, err)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("file info %s: %w", path, err)
		}

		if err := root.WriteFile(path, sanatized, info.Mode()); err != nil {
			return fmt.Errorf("write file %s: %w", path, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
