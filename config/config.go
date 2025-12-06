package config

import (
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type TelegramBot struct {
	Token         string        `yaml:"token" validate:"required"`
	Chats         []Chat        `yaml:"chats"`
	UpdateTimeout time.Duration `yaml:"update_timeout" validate:"required"`
}

type Chat struct {
	Id       int64 `yaml:"id" validate:"required"`
	OriginId int64 `yaml:"origin_id"`
}

type CommentGenerator struct {
	Url      string `yaml:"url" validate:"required"`
	Username string `yaml:"username" validate:"required"`
	Password string `yaml:"password" validate:"required"`
}

type Config struct {
	TelegramBot      TelegramBot      `yaml:"telegram_bot"`
	CommentGenerator CommentGenerator `yaml:"comment_generator"`
	UseDebugMode     bool             `yaml:"use_debug_mode"`
}

func Load() (cfg *Config, err error) {
	f, err := os.Open(defineConfigPath())
	if err != nil {
		return nil, errors.WithMessage(err, "open config file")
	}
	defer func() {
		if cErr := f.Close(); cErr != nil && err == nil {
			err = errors.WithMessage(cErr, "close config file")
		}
	}()

	cfg = new(Config)
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, errors.WithMessage(err, "decode config yml file")
	}
	if err := validator.New().Struct(cfg); err != nil {
		return nil, errors.WithMessage(err, "validate config")
	}

	return cfg, nil
}

func defineConfigPath() string {
	isDev := os.Getenv("APP_MODE") == "dev"
	if isDev {
		return "./config/config_dev.yml"
	}
	return "./config.yml"
}
