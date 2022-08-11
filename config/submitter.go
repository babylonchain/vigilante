package config

// SubmitterConfig defines configuration for the gRPC-web server.
type SubmitterConfig struct {
	Placeholder string `mapstructure:"placeholder"`
}

func DefaultSubmitterConfig() SubmitterConfig {
	return SubmitterConfig{
		Placeholder: "submitterconfig",
	}
}
