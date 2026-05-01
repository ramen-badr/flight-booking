package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env        string     `yaml:"env" env-default:"local"`
	HTTPServer HTTPServer `yaml:"http-server"`
	Storage    Storage    `yaml:"storage"`
}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:8081"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

type Storage struct {
	Address  string `yaml:"address" env-default:"localhost:5432"`
	User     string `env:"DB_USER" env-required:"true"`
	Name     string `env:"DB_NAME" env-required:"true"`
	Password string `env:"DB_PASSWORD" env-required:"true"`
}

func MustLoad() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("environment variable CONFIG_PATH not set")
	}

	if _, err = os.Stat(configPath); os.IsNotExist(err) {
		log.Fatal("config file does not exist:", configPath)
	}

	var config Config

	if err = cleanenv.ReadConfig(configPath, &config); err != nil {
		log.Fatal("cannot read config:", err)
	}

	return &config
}
