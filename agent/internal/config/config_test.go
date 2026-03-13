package config

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_LoadFromJSONAndArgs_ValuesLoadedFromJSON(t *testing.T) {
	dir := setupTempDir(t)
	writeConfigJSON(t, dir, Config{
		DatabasusHost: "http://json-host:4005",
		DbID:          "json-db-id",
		Token:         "json-token",
	})

	cfg := &Config{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg.LoadFromJSONAndArgs(fs, []string{})

	assert.Equal(t, "http://json-host:4005", cfg.DatabasusHost)
	assert.Equal(t, "json-db-id", cfg.DbID)
	assert.Equal(t, "json-token", cfg.Token)
}

func Test_LoadFromJSONAndArgs_ValuesLoadedFromArgs_WhenNoJSON(t *testing.T) {
	setupTempDir(t)

	cfg := &Config{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg.LoadFromJSONAndArgs(fs, []string{
		"--databasus-host", "http://arg-host:4005",
		"--db-id", "arg-db-id",
		"--token", "arg-token",
	})

	assert.Equal(t, "http://arg-host:4005", cfg.DatabasusHost)
	assert.Equal(t, "arg-db-id", cfg.DbID)
	assert.Equal(t, "arg-token", cfg.Token)
}

func Test_LoadFromJSONAndArgs_ArgsOverrideJSON(t *testing.T) {
	dir := setupTempDir(t)
	writeConfigJSON(t, dir, Config{
		DatabasusHost: "http://json-host:4005",
		DbID:          "json-db-id",
		Token:         "json-token",
	})

	cfg := &Config{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg.LoadFromJSONAndArgs(fs, []string{
		"--databasus-host", "http://arg-host:9999",
		"--db-id", "arg-db-id-override",
		"--token", "arg-token-override",
	})

	assert.Equal(t, "http://arg-host:9999", cfg.DatabasusHost)
	assert.Equal(t, "arg-db-id-override", cfg.DbID)
	assert.Equal(t, "arg-token-override", cfg.Token)
}

func Test_LoadFromJSONAndArgs_PartialArgsOverrideJSON(t *testing.T) {
	dir := setupTempDir(t)
	writeConfigJSON(t, dir, Config{
		DatabasusHost: "http://json-host:4005",
		DbID:          "json-db-id",
		Token:         "json-token",
	})

	cfg := &Config{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg.LoadFromJSONAndArgs(fs, []string{
		"--databasus-host", "http://arg-host-only:4005",
	})

	assert.Equal(t, "http://arg-host-only:4005", cfg.DatabasusHost)
	assert.Equal(t, "json-db-id", cfg.DbID)
	assert.Equal(t, "json-token", cfg.Token)
}

func Test_SaveToJSON_ConfigSavedCorrectly(t *testing.T) {
	setupTempDir(t)

	cfg := &Config{
		DatabasusHost: "http://save-host:4005",
		DbID:          "save-db-id",
		Token:         "save-token",
	}

	err := cfg.SaveToJSON()
	require.NoError(t, err)

	saved := readConfigJSON(t)

	assert.Equal(t, "http://save-host:4005", saved.DatabasusHost)
	assert.Equal(t, "save-db-id", saved.DbID)
	assert.Equal(t, "save-token", saved.Token)
}

func Test_SaveToJSON_AfterArgsOverrideJSON_SavedFileContainsMergedValues(t *testing.T) {
	dir := setupTempDir(t)
	writeConfigJSON(t, dir, Config{
		DatabasusHost: "http://json-host:4005",
		DbID:          "json-db-id",
		Token:         "json-token",
	})

	cfg := &Config{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg.LoadFromJSONAndArgs(fs, []string{
		"--databasus-host", "http://override-host:9999",
	})

	err := cfg.SaveToJSON()
	require.NoError(t, err)

	saved := readConfigJSON(t)

	assert.Equal(t, "http://override-host:9999", saved.DatabasusHost)
	assert.Equal(t, "json-db-id", saved.DbID)
	assert.Equal(t, "json-token", saved.Token)
}

func setupTempDir(t *testing.T) string {
	t.Helper()

	origDir, err := os.Getwd()
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))

	t.Cleanup(func() { os.Chdir(origDir) })

	return dir
}

func writeConfigJSON(t *testing.T, dir string, cfg Config) {
	t.Helper()

	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(dir+"/"+configFileName, data, 0o644))
}

func readConfigJSON(t *testing.T) Config {
	t.Helper()

	data, err := os.ReadFile(configFileName)
	require.NoError(t, err)

	var cfg Config
	require.NoError(t, json.Unmarshal(data, &cfg))

	return cfg
}
