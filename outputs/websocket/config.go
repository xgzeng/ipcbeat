package websocket

// configuration for websocket output
type Config struct {
	// old pretty settings to use if no codec is configured
	Addr      string `config:"addr"`
	BatchSize int
}

var defaultConfig = Config{}
