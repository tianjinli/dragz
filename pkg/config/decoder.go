package config

import "github.com/go-viper/mapstructure/v2"

// YamlTagDecoder use yaml struct tags when decoding
func YamlTagDecoder(c *mapstructure.DecoderConfig) { c.TagName = "yaml" }
