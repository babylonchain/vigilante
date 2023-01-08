package config

const (
	defaultCheckpointBuffer = 100
)

// MonitorConfig defines the server's basic configuration
type MonitorConfig struct {
	// Max number of checkpoints in the buffer
	CheckpointBuffer uint64 `mapstructure:"checkpoint-buffer"`
}

func (cfg *MonitorConfig) Validate() error {
	return nil
}

func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		CheckpointBuffer: defaultCheckpointBuffer,
	}
}
