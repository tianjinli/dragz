package config

import (
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

var compile = regexp.MustCompile(`\$\{([^}]+)}`)

// ExpandEnv replaces `${ENV_VAR:default}` and `${ENV_VAR}` with environment values.
// `${some.key:default}` and `${some.key}` work the same way for Viper keys.
// If no default is provided, the placeholder is set to empty.
func ExpandEnv(v *viper.Viper) *viper.Viper {
	pending := map[string]string{}

	for _, key := range v.AllKeys() {
		if val, ok := v.Get(key).(string); ok {
			expanded, needsUpdate, needsNested := expandPlaceholders(val)
			if needsUpdate {
				v.Set(key, expanded)
			} else if needsNested {
				pending[key] = expanded
			}
		}
	}
	for key, val := range pending {
		expanded := resolveNested(val, v)
		v.Set(key, expanded)
	}

	return v
}

// expandPlaceholders scans the input string for placeholders like `${ENV_VAR:default}`
// or `${ENV_VAR}`, and replaces them with environment variable values or defaults.
// It also detects nested placeholders like `${some.key}` that must be resolved later.
func expandPlaceholders(input string) (string, bool, bool) {
	matches := compile.FindAllStringSubmatch(input, -1)
	needsUpdate := false
	needsNested := false // flag to indicate if nested keys remain

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		raw := match[1] // the content inside `${...}`
		var replacement string

		if strings.Contains(raw, ":") {
			// Case 1: placeholder has a default value, e.g. ${ENV_VAR:default}
			parts := strings.SplitN(raw, ":", 2)
			env := parts[0]
			def := parts[1]
			if strings.Contains(env, ".") {
				// If the name contains '.', treat it as a nested config key
				needsNested = true
				continue
			}
			replacement = os.Getenv(env)
			if replacement == "" {
				replacement = def // fallback to default if empty
			}
		} else {
			// Case 2: placeholder without default, e.g. ${ENV_VAR}
			if strings.Contains(raw, ".") {
				// Nested key reference, defer resolution
				needsNested = true
				continue
			}
			replacement = os.Getenv(raw)
		}
		needsUpdate = true
		// Replace the full placeholder `${...}` with the resolved value
		input = strings.ReplaceAll(input, match[0], replacement)
	}
	// Return the expanded string and whether nested resolution is still needed
	return input, needsUpdate, needsNested
}

// resolveNested handles placeholders that reference other Viper keys,
// e.g. `${some.key}`. It looks up the key in Viper and replaces it.
func resolveNested(input string, v *viper.Viper) string {
	matches := compile.FindAllStringSubmatch(input, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		refKey := match[1] // the referenced config key
		replacement := v.GetString(refKey)
		// Replace `${some.key}` with the actual config value
		input = strings.ReplaceAll(input, match[0], replacement)
	}

	return input
}

// ReverseReadInConfig read config file, then restore previous values to keep them.
func ReverseReadInConfig(v *viper.Viper) error {
	oldMap := v.AllSettings()
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.MergeConfigMap(oldMap); err != nil {
		return err
	}
	ExpandEnv(v)
	return nil
}

// ReverseReadConfig read config from io.Reader,  then restore previous values to keep them.
func ReverseReadConfig(v *viper.Viper, in io.Reader) error {
	oldMap := v.AllSettings()
	if err := v.ReadConfig(in); err != nil {
		return err
	}
	if err := v.MergeConfigMap(oldMap); err != nil {
		return err
	}
	ExpandEnv(v)
	return nil
}

// ReverseMergeViper Merge settings from old Viper into new Viper.
func ReverseMergeViper(newV *viper.Viper, oldV *viper.Viper) error {
	oldMap := oldV.AllSettings()
	if err := newV.MergeConfigMap(oldMap); err != nil {
		return err
	}
	ExpandEnv(newV)
	return nil
}
