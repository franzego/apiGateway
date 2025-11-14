package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	RabbitMq RabbitMqConfig
}

type RabbitMqConfig struct {
	Url        string
	EmailQueue string
	PushQueue  string
	Exchange   string
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	viper.SetDefault("rabbit.mq.email_queue", "email.queue")
	viper.SetDefault("rabbit.mq.push_queue", "push_queue")
	viper.SetDefault("rabbitmq.exchange", "notification.direct")

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("an error occurred reading configuration file")

	}
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("an error unmarshalling the config")
	}
	return &config, nil
}
