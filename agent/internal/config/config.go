package config

import (
	"encoding/json"
	"flag"
	"os"

	"databasus-agent/internal/logger"
)

var log = logger.GetLogger()

const configFileName = "databasus.json"

type Config struct {
	DatabasusHost string `json:"databasusHost"`
	DbID          string `json:"dbId"`
	Token         string `json:"token"`

	flags parsedFlags
}

// LoadFromJSONAndArgs reads databasus.json into the struct
// and overrides JSON values with any explicitly provided CLI flags.
func (c *Config) LoadFromJSONAndArgs(fs *flag.FlagSet, args []string) {
	c.loadFromJSON()
	c.initSources()

	c.flags.host = fs.String(
		"databasus-host",
		"",
		"Databasus server URL (e.g. http://your-server:4005)",
	)
	c.flags.dbID = fs.String("db-id", "", "Database ID")
	c.flags.token = fs.String("token", "", "Agent token")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	c.applyFlags()
	log.Info("========= Loading config ============")
	c.logConfigSources()
	log.Info("========= Config has been loaded ====")
}

// SaveToJSON writes the current struct to databasus.json.
func (c *Config) SaveToJSON() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configFileName, data, 0o644)
}

func (c *Config) loadFromJSON() {
	data, err := os.ReadFile(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("No databasus.json found, will create on save")
			return
		}

		log.Warn("Failed to read databasus.json", "error", err)

		return
	}

	if err := json.Unmarshal(data, c); err != nil {
		log.Warn("Failed to parse databasus.json", "error", err)

		return
	}

	log.Info("Configuration loaded from " + configFileName)
}

func (c *Config) initSources() {
	c.flags.sources = map[string]string{
		"databasus-host": "not configured",
		"db-id":          "not configured",
		"token":          "not configured",
	}

	if c.DatabasusHost != "" {
		c.flags.sources["databasus-host"] = configFileName
	}

	if c.DbID != "" {
		c.flags.sources["db-id"] = configFileName
	}

	if c.Token != "" {
		c.flags.sources["token"] = configFileName
	}
}

func (c *Config) applyFlags() {
	if c.flags.host != nil && *c.flags.host != "" {
		c.DatabasusHost = *c.flags.host
		c.flags.sources["databasus-host"] = "command line args"
	}

	if c.flags.dbID != nil && *c.flags.dbID != "" {
		c.DbID = *c.flags.dbID
		c.flags.sources["db-id"] = "command line args"
	}

	if c.flags.token != nil && *c.flags.token != "" {
		c.Token = *c.flags.token
		c.flags.sources["token"] = "command line args"
	}
}

func (c *Config) logConfigSources() {
	log.Info(
		"databasus-host",
		"value",
		c.DatabasusHost,
		"source",
		c.flags.sources["databasus-host"],
	)
	log.Info("db-id", "value", c.DbID, "source", c.flags.sources["db-id"])
	log.Info("token", "value", maskSensitive(c.Token), "source", c.flags.sources["token"])
}

func maskSensitive(value string) string {
	if value == "" {
		return "(not set)"
	}

	visibleLen := max(len(value)/4, 1)

	return value[:visibleLen] + "***"
}
