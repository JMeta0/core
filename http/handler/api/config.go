package api

import (
	"io"
	"net/http"
	"time"

	"github.com/datarhei/core/v16/config"
	cfgstore "github.com/datarhei/core/v16/config/store"
	cfgvars "github.com/datarhei/core/v16/config/vars"
	"github.com/datarhei/core/v16/encoding/json"
	"github.com/datarhei/core/v16/http/api"

	"github.com/labstack/echo/v4"
)

// The ConfigHandler type provides handler functions for reading and manipulating
// the current config.
type ConfigHandler struct {
	store cfgstore.Store
}

// NewConfig return a new Config type. You have to provide a valid config store.
func NewConfig(store cfgstore.Store) *ConfigHandler {
	return &ConfigHandler{
		store: store,
	}
}

// redactedSecret is the placeholder that is returned instead of a secret value.
// If a client sends this value back unchanged, the currently stored value is
// kept instead of being overwritten.
const redactedSecret = "***"

// maskSecrets replaces secret values with the redaction placeholder so that
// credentials are not leaked through the config API. Only values that are not
// required by the web UI to derive other configs (see restoreSecrets) are
// redacted here.
func maskSecrets(d *config.Data) {
	d.API.Auth.JWT.Secret = redactedSecret
	d.Service.Token = redactedSecret

	for i := range d.Storage.S3 {
		d.Storage.S3[i].SecretAccessKey = redactedSecret
		d.Storage.S3[i].Auth.Password = redactedSecret
	}
}

// restoreSecrets restores redacted secret values from the original (stored)
// config. Values that were changed by the client (i.e. that differ from the
// placeholder) are left untouched. This keeps clients that round-trip a
// redacted config (like the web UI) from accidentally clearing secrets.
func restoreSecrets(d, original *config.Data) {
	if d.API.Auth.JWT.Secret == redactedSecret {
		d.API.Auth.JWT.Secret = original.API.Auth.JWT.Secret
	}

	if d.Service.Token == redactedSecret {
		d.Service.Token = original.Service.Token
	}

	for i := range d.Storage.S3 {
		for j := range original.Storage.S3 {
			if d.Storage.S3[i].Name != original.Storage.S3[j].Name {
				continue
			}

			if d.Storage.S3[i].SecretAccessKey == redactedSecret {
				d.Storage.S3[i].SecretAccessKey = original.Storage.S3[j].SecretAccessKey
			}

			if d.Storage.S3[i].Auth.Password == redactedSecret {
				d.Storage.S3[i].Auth.Password = original.Storage.S3[j].Auth.Password
			}
		}
	}
}

// Get returns the currently active Restreamer configuration
// @Summary Retrieve the currently active Restreamer configuration
// @Description Retrieve the currently active Restreamer configuration
// @Tags v16.7.2
// @ID config-3-get
// @Produce json
// @Success 200 {object} api.Config
// @Security ApiKeyAuth
// @Router /api/v3/config [get]
func (p *ConfigHandler) Get(c echo.Context) error {
	cfg := p.store.GetActive()

	apicfg := api.Config{}
	apicfg.Unmarshal(cfg)

	maskSecrets(&apicfg.Config.Data)

	return c.JSON(http.StatusOK, apicfg)
}

// Set will set the given configuration as new active configuration
// @Summary Update the current Restreamer configuration
// @Description Update the current Restreamer configuration by providing a complete or partial configuration. Fields that are not provided will not be changed.
// @Tags v16.7.2
// @ID config-3-set
// @Accept json
// @Produce json
// @Param config body api.SetConfig true "Restreamer configuration"
// @Success 200 {string} string
// @Failure 400 {object} api.Error
// @Failure 409 {object} api.ConfigError
// @Security ApiKeyAuth
// @Router /api/v3/config [put]
func (p *ConfigHandler) Set(c echo.Context) error {
	version := api.ConfigVersion{}

	req := c.Request()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", err)
	}

	if err := json.Unmarshal(body, &version); err != nil {
		return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", json.FormatError(body, err))
	}

	cfg := p.store.Get()
	cfgActive := p.store.GetActive()

	// Keep the stored secrets around so that redacted values coming back from
	// a client can be restored instead of overwriting them.
	original := cfg.Clone()

	// Copy the timestamp of when this config has been used
	cfg.LoadedAt = cfgActive.LoadedAt

	// For each version, set the current config as default config value. This will
	// allow to set a partial config without destroying the other values.
	if version.Version == 1 {
		// Downgrade to v1 in order to have a populated v1 config
		v1SetConfig := api.NewSetConfigV1(cfg)

		if err := json.Unmarshal(body, &v1SetConfig); err != nil {
			return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", json.FormatError(body, err))
		}

		if err := c.Validate(v1SetConfig); err != nil {
			return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", err)
		}

		// Merge it into the current config
		v1SetConfig.MergeTo(cfg)
	} else if version.Version == 2 {
		// Downgrade to v2 in order to have a populated v2 config
		v2SetConfig := api.NewSetConfigV2(cfg)

		if err := json.Unmarshal(body, &v2SetConfig); err != nil {
			return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", json.FormatError(body, err))
		}

		if err := c.Validate(v2SetConfig); err != nil {
			return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", err)
		}

		// Merge it into the current config
		v2SetConfig.MergeTo(cfg)
	} else if version.Version == 3 {
		v3SetConfig := api.NewSetConfig(cfg)

		if err := json.Unmarshal(body, &v3SetConfig); err != nil {
			return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", json.FormatError(body, err))
		}

		if err := c.Validate(v3SetConfig); err != nil {
			return api.Err(http.StatusBadRequest, "Invalid JSON", "%s", err)
		}

		// Merge it into the current config
		v3SetConfig.MergeTo(cfg)
	} else {
		return api.Err(http.StatusBadRequest, "Invalid config version", "version %d", version.Version)
	}

	// Restore any secrets that a client sent back redacted (see maskSecrets).
	restoreSecrets(&cfg.Data, &original.Data)

	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = cfg.CreatedAt

	// Now we make a copy from the config and merge it with the environment
	// variables. If this configuration is valid, we will store the un-merged
	// one to disk.

	mergedConfig := cfg.Clone()
	mergedConfig.Merge()

	// Validate the new merged config
	mergedConfig.Validate(true)
	if mergedConfig.HasErrors() {
		errors := make(map[string][]string)

		mergedConfig.Messages(func(level string, v cfgvars.Variable, message string) {
			if level != "error" {
				return
			}

			errors[v.Name] = append(errors[v.Name], message)
		})

		return c.JSON(http.StatusConflict, errors)
	}

	// Save the new config
	if err := p.store.Set(cfg); err != nil {
		return api.Err(http.StatusBadRequest, "Failed to store config", "%s", err)
	}

	// Set the new and merged config as active config
	if err := p.store.SetActive(mergedConfig); err != nil {
		return api.Err(http.StatusBadRequest, "Failed to activate config", "%s", err)
	}

	return c.JSON(http.StatusOK, "OK")
}

// Reload will reload the currently active configuration
// @Summary Reload the currently active configuration
// @Description Reload the currently active configuration. This will trigger a restart of the Core.
// @Tags v16.7.2
// @ID config-3-reload
// @Produce json
// @Success 200 {string} string
// @Security ApiKeyAuth
// @Router /api/v3/config/reload [get]
func (p *ConfigHandler) Reload(c echo.Context) error {
	p.store.Reload()

	return c.JSON(http.StatusOK, "OK")
}
