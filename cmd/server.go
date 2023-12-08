package cmd

import (
	"crypto/ecdsa"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/chainflag/eth-faucet/internal/chain"
	"github.com/chainflag/eth-faucet/internal/server"
)

var (
	appVersion = "v1.2.1"
	chainIDMap = map[string]int{"scroll-sepolia": 534351, "polygon-mumbai": 80001, "arbitrum-goerli": 421613, "optimism-goerli": 420, "base-goerli": 84531}

	httpPortFlag   = flag.Int("httpport", 8080, "Listener port to serve HTTP connection")
	proxyCountFlag = flag.Int("proxycount", 0, "Count of reverse proxies in front of the server")
	queueCapFlag   = flag.Int("queuecap", 10, "Maximum transactions waiting to be sent")
	versionFlag    = flag.Bool("version", true, "Print version number")

	contractFlag    = flag.String("faucet.contract", "", "Faucet Contract Address")
	ethAmountFlag   = flag.Int("faucet.ethamount", 0, "Number of ETH to transfer per user request")
	usdtAmountFlag  = flag.Int("faucet.usdtamount", 0, "Number of USDT to transfer per user request")
	usdcAmountFlag  = flag.Int("faucet.usdcamount", 0, "Number of USDC to transfer per user request")
	intervalFlag    = flag.Int("faucet.minutes", 1, "Number of minutes to wait between funding rounds")
	gasPriceFlag    = flag.Int("faucet.gasprice", 0, "GasPrice")
	networkNameFlag = flag.String("faucet.networkname", "testnet", "Network name to display on the frontend")

	keyJsonFlag    = flag.String("wallet.keyjson", os.Getenv("KEYSTORE"), "Keystore file to fund user requests with")
	keyPassFlag    = flag.String("wallet.keypass", "password.txt", "Passphrase text file to decrypt keystore")
	privateKeyFlag = flag.String("wallet.privatekey", os.Getenv("PRIVATE_KEY"), "Private key hex to fund user requests with")
	providerFlag   = flag.String("wallet.provider", os.Getenv("WEB3_PROVIDER"), "Endpoint for Ethereum JSON-RPC connection")
)

func init() {
	flag.Parse()
	if *versionFlag {
		fmt.Println(appVersion)
	}
}

func Execute() {
	privateKey, err := getPrivateKeyFromFlags()
	if err != nil {
		panic(fmt.Errorf("failed to read private key: %w", err))
	}
	var chainID *big.Int
	if value, ok := chainIDMap[strings.ToLower(*networkNameFlag)]; ok {
		chainID = big.NewInt(int64(value))
	}

	txBuilder, err := chain.NewTxBuilder(*providerFlag, privateKey, chainID)
	if err != nil {
		panic(fmt.Errorf("cannot connect to web3 provider: %w", err))
	}
	config := server.NewConfig(*httpPortFlag, *proxyCountFlag, *queueCapFlag, *contractFlag, *ethAmountFlag, *usdtAmountFlag, *usdcAmountFlag, *intervalFlag, *gasPriceFlag, *networkNameFlag)
	go server.NewServer(txBuilder, config).Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func getPrivateKeyFromFlags() (*ecdsa.PrivateKey, error) {
	if *privateKeyFlag != "" {
		hexkey := *privateKeyFlag
		if chain.Has0xPrefix(hexkey) {
			hexkey = hexkey[2:]
		}
		return crypto.HexToECDSA(hexkey)
	} else if *keyJsonFlag == "" {
		return nil, errors.New("missing private key or keystore")
	}

	keyfile, err := chain.ResolveKeyfilePath(*keyJsonFlag)
	if err != nil {
		return nil, err
	}
	password, err := os.ReadFile(*keyPassFlag)
	if err != nil {
		return nil, err
	}

	return chain.DecryptKeyfile(keyfile, strings.TrimRight(string(password), "\r\n"))
}
