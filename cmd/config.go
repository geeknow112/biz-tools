package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Platforms map[string]PlatformConfig `yaml:"platforms"`
	Crawl     CrawlConfig               `yaml:"crawl"`
	Outreach  OutreachConfig            `yaml:"outreach"`
}

type PlatformConfig struct {
	Repo        string `yaml:"repo"`
	URL         string `yaml:"url"`
	Username    string `yaml:"username"`
	AppPassword string `yaml:"app_password"`

	// X (Twitter) API v2 credentials — OAuth 1.0a User Context.
	// Generate these in the X Developer Portal (App settings > Keys and
	// tokens) after enabling OAuth 1.0a with Read and Write permissions.
	APIKey            string `yaml:"api_key"`
	APISecret         string `yaml:"api_secret"`
	AccessToken       string `yaml:"access_token"`
	AccessTokenSecret string `yaml:"access_token_secret"`
}

type CrawlConfig struct {
	SerpAPIKey string `yaml:"serpapi_key"`
}

type OutreachConfig struct {
	SenderName      string `yaml:"sender_name"`
	SenderEmail     string `yaml:"sender_email"`
	SenderCompany   string `yaml:"sender_company"`
	MessageTemplate string `yaml:"message_template"`
}

func loadConfig() (*Config, error) {
	// Look for config.yaml in current dir or executable dir
	configPaths := []string{
		"config.yaml",
		filepath.Join(filepath.Dir(os.Args[0]), "config.yaml"),
	}

	var data []byte
	var err error
	for _, path := range configPaths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("config.yaml not found")
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return &config, nil
}
