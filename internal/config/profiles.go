package config

import "fmt"

// AddProfile adds or replaces a named profile and fills safe defaults.
func AddProfile(cfg *Config, name string, p Profile) error {
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.AccountURL == "" {
		return fmt.Errorf("account_url is required")
	}
	if err := ValidateAccountURL(p.AccountURL); err != nil {
		return err
	}
	if err := ValidateSecretRef(p.SecretRef); err != nil {
		return err
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if p.AuthType == "" {
		p.AuthType = "oauth2"
	}
	if p.SecretRef == "" {
		p.SecretRef = "keyring:wsectl/" + name
	}
	cfg.Profiles[name] = p
	if cfg.CurrentProfile == "" {
		cfg.CurrentProfile = name
	}
	return nil
}

// RemoveProfile deletes a named profile from the configuration.
func RemoveProfile(cfg *Config, name string) error {
	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}
	delete(cfg.Profiles, name)
	if cfg.CurrentProfile == name {
		cfg.CurrentProfile = "default"
	}
	return nil
}
