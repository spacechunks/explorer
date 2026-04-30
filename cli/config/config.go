package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Oudwins/zog"
	"github.com/goccy/go-yaml"
)

type Config struct {
	Version string `json:"version"`
	Chunk   Chunk  `json:"chunk"`
}

type Chunk struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Thumbnail   string   `json:"thumbnail"`
	Tags        []string `json:"tags"`
	Flavors     []Flavor `json:"flavors"`
}

type Flavor struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	MinecraftVersion string `json:"minecraftVersion"`
	Path             string `json:"path"`
}

var schemaV1Alpha1 = zog.Struct(zog.Shape{
	"version": zog.String().Match(regexp.MustCompile("v1alpha1")).Required(),
	"chunk": zog.Struct(zog.Shape{
		"name":        zog.String().Max(50).Required(),
		"description": zog.String().Max(100).Required(),
		"thumbnail":   zog.String().Optional(),
		"tags":        zog.Slice(zog.String()).Max(4).Optional(),
		"flavors": zog.Slice(zog.Struct(zog.Shape{
			"name":             zog.String().Max(25).Required(),
			"version":          zog.String().Required(),
			"minecraftVersion": zog.String().Required(),
			"path":             zog.String().Required(),
		})),
	}),
})

func Validate(cfg Config) map[string][]string {
	var flattened map[string][]string
	issues := schemaV1Alpha1.Validate(&cfg)
	if len(issues) > 0 {
		flattened = zog.Issues.Flatten(issues)
	}
	return flattened
}

func ReadWithResolvedPaths(configPath string) (Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.Chunk.Thumbnail != "" {
		resolvedThmb, err := resolvePath(configPath, cfg.Chunk.Thumbnail)
		if err != nil {
			return Config{}, fmt.Errorf("resolve thumbnail configPath: %w", err)
		}
		cfg.Chunk.Thumbnail = resolvedThmb
	}

	// resolve to absolute paths, because publish could be called from
	// anywhere in the filesystem and flavor paths _could_ be relative
	// to the directory where the .chunk.yaml lives.
	for idx, f := range cfg.Chunk.Flavors {
		resolved, err := resolvePath(configPath, f.Path)
		if err != nil {
			return Config{}, fmt.Errorf("resolve configPath: %w", err)
		}

		cfg.Chunk.Flavors[idx].Path = resolved
	}

	return cfg, nil
}

func resolvePath(configPath string, elemPath string) (string, error) {
	if filepath.IsAbs(elemPath) {
		return elemPath, nil
	}

	abs, err := filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("abs: %w", err)
	}

	return filepath.Join(filepath.Dir(abs), elemPath), nil
}
