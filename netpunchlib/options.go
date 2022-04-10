package netpunchlib

type Config struct {
	connMW []ConnectionMiddleware
}

type Option func(cfg *Config)

func newConfig(options ...Option) *Config {
	cfg := new(Config)
	for _, o := range options {
		o(cfg)
	}
	return cfg
}

func (c *Config) wrapConnection(conn Connection) Connection { //nolint:ireturn
	for _, mw := range c.connMW {
		conn = mw(conn)
	}
	return conn
}

func ConnOption(mw ...ConnectionMiddleware) Option {
	return func(cfg *Config) {
		cfg.connMW = append(cfg.connMW, mw...)
	}
}
