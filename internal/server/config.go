package server

type Config struct {
	httpPort        int
	proxyCount      int
	queueCap        int
	contractAddress string
	ethAmount       int
	usdtAmount      int
	usdcAmount      int
	interval        int
	gasPrice        int
	networkName     string
}

func NewConfig(httpPort int, proxyCount int, queueCap int, contractAddress string, ethAmount int, usdtAmount int, usdcAmount int, interval int, gasPrice int, networkName string) *Config {
	return &Config{
		httpPort:        httpPort,
		proxyCount:      proxyCount,
		queueCap:        queueCap,
		contractAddress: contractAddress,
		ethAmount:       ethAmount,
		usdtAmount:      usdtAmount,
		usdcAmount:      usdcAmount,
		interval:        interval,
		gasPrice:        gasPrice,
		networkName:     networkName,
	}
}
