package serverconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
	walked := make(map[string]struct{})

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

		// we need to track, which files we walked, because later we have to check
		// if a file that should be sanatized was present. if not, we need to set a
		// default value in order to make sure, that there is always a working config
		// present.
		walked[path] = struct{}{}

		return nil
	}); err != nil {
		return fmt.Errorf("walk: %w", err)
	}

	for _, p := range []string{"config/paper-global.yml", "server.properties"} {
		if _, found := walked[p]; found {
			continue
		}

		if err := writeDefaultConfig(root, p); err != nil {
			return fmt.Errorf("write default: %w", err)
		}
	}

	return nil
}

func writeDefaultConfig(root *os.Root, path string) error {
	def := ""
	switch path {
	case "config/paper-global.yml":
		def = defaultPaperGlobalStr
	case "server.properties":
		def = defaultServerPropertiesStr
	}

	if def == "" {
		return errors.New("no default configuration found")
	}

	dir := filepath.Dir(path)

	if err := root.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("mkdir all: %w", err)
	}

	if err := root.WriteFile(path, []byte(def), os.ModePerm); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
