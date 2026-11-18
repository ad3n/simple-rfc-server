package config

import (
	"fmt"
	"io"
	"math"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

var namePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,29}$`)

type (
	Config struct {
		Functions []Function `yaml:"functions"`
		Port      int        `yaml:"port"`
	}

	Function struct {
		Name       string      `yaml:"name"`
		Parameters []Parameter `yaml:"parameters"`
	}

	Parameter struct {
		Value     any    `yaml:"value"`
		Name      string `yaml:"name"`
		Type      string `yaml:"type"`
		Direction string `yaml:"direction"`
		From      string `yaml:"from"`
		Length    int    `yaml:"length"`
	}
)

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("config must contain exactly one YAML document")
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if len(c.Functions) == 0 {
		return fmt.Errorf("functions cannot be empty")
	}

	names := make(map[string]bool)
	for _, fn := range c.Functions {
		if !namePattern.MatchString(fn.Name) || names[fn.Name] || fn.Name == "RFC_GET_FUNCTION_INTERFACE" {
			return fmt.Errorf("invalid or duplicate function %q", fn.Name)
		}

		names[fn.Name] = true
		params := make(map[string]Parameter)
		for _, p := range fn.Parameters {
			if !namePattern.MatchString(p.Name) {
				return fmt.Errorf("%s: invalid parameter %q", fn.Name, p.Name)
			}

			if _, ok := params[p.Name]; ok {
				return fmt.Errorf("%s: duplicate parameter %s", fn.Name, p.Name)
			}

			params[p.Name] = p
			switch p.Direction {
			case "import", "export", "changing":
			default:
				return fmt.Errorf("%s.%s: direction must be import, export, or changing", fn.Name, p.Name)
			}

			switch p.Type {
			case "CHAR":
				if p.Length < 1 || p.Length > 65535 {
					return fmt.Errorf("%s.%s: CHAR length must be 1..65535", fn.Name, p.Name)
				}
			case "STRING", "INT", "FLOAT":
				if p.Length != 0 {
					return fmt.Errorf("%s.%s: length is only valid for CHAR", fn.Name, p.Name)
				}
			default:
				return fmt.Errorf("%s.%s: unsupported type %q", fn.Name, p.Name, p.Type)
			}

			if p.Direction == "import" && (p.Value != nil || p.From != "") {
				return fmt.Errorf("%s.%s: import cannot have value or from", fn.Name, p.Name)
			}

			if p.Value != nil && p.From != "" {
				return fmt.Errorf("%s.%s: use either value or from", fn.Name, p.Name)
			}

			if p.Value != nil {
				if err := p.ValidateValue(p.Value); err != nil {
					return fmt.Errorf("%s.%s: %w", fn.Name, p.Name, err)
				}
			}
		}

		for _, p := range fn.Parameters {
			if p.From == "" {
				continue
			}

			src, ok := params[p.From]
			if !ok || src.Direction == "export" || src.Type != p.Type || src.Length != p.Length {
				return fmt.Errorf("%s.%s: from must reference an import/changing parameter of the same type and length", fn.Name, p.Name)
			}
		}
	}

	return nil
}

func (p Parameter) ValidateValue(value any) error {
	switch p.Type {
	case "CHAR", "STRING":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("value must be a string")
		}

		units := 0
		for _, r := range s {
			units++
			if r > 0xffff {
				units++
			}
		}

		if p.Type == "CHAR" && units > p.Length {
			return fmt.Errorf("value exceeds CHAR length")
		}
	case "INT":
		n, ok := value.(int)
		if !ok || int64(n) < -2147483648 || int64(n) > 2147483647 {
			return fmt.Errorf("value must be a signed 32-bit integer")
		}
	case "FLOAT":
		switch value.(type) {
		case int:
		case float64:
			n := value.(float64)
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return fmt.Errorf("value must be finite")
			}
		default:
			return fmt.Errorf("value must be numeric")
		}
	}

	return nil
}
