package server

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LK4D4/trylock"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/negroni"

	"github.com/chainflag/eth-faucet/internal/chain"
	"github.com/chainflag/eth-faucet/web"
)

type Server struct {
	chain.TxBuilder
	mutex trylock.Mutex
	cfg   *Config
	queue chan string
}

func NewServer(builder chain.TxBuilder, cfg *Config) *Server {
	return &Server{
		TxBuilder: builder,
		cfg:       cfg,
		queue:     make(chan string, cfg.queueCap),
	}
}

func (s *Server) setupRouter() *http.ServeMux {
	router := http.NewServeMux()
	router.Handle("/", http.FileServer(web.Dist()))
	limiter := NewLimiter(s.cfg.proxyCount, time.Duration(s.cfg.interval)*time.Minute)
	router.Handle("/api/claim", negroni.New(limiter, negroni.Wrap(s.handleClaim())))
	router.Handle("/api/info", s.handleInfo())

	return router
}

func (s *Server) Run() {
	go func() {
		ticker := time.NewTicker(time.Second)
		for range ticker.C {
			s.consumeQueue()
		}
	}()

	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())
	n.UseHandler(s.setupRouter())
	log.Infof("Starting http server %d", s.cfg.httpPort)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(s.cfg.httpPort), n))
}

func (s *Server) consumeQueue() {
	if len(s.queue) == 0 {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	for len(s.queue) != 0 {
		address := <-s.queue
		txHash, err := s.Transfer(context.Background(), address, chain.EtherToWei(int64(s.cfg.ethAmount)), big.NewInt(int64(s.cfg.gasPrice)))
		if err != nil {
			log.WithError(err).Error("Failed to handle transaction in the queue")
		} else {
			log.WithFields(log.Fields{
				"txHash":  txHash,
				"address": address,
			}).Info("Consume from queue successfully")
		}
	}
}

func (s *Server) handleClaim() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.NotFound(w, r)
			return
		}

		// The error always be nil since it has already been handled in limiter
		address, _ := readAddress(r)
		// Try to lock mutex if the work queue is empty
		if len(s.queue) != 0 || !s.mutex.TryLock() {
			select {
			case s.queue <- address:
				log.WithFields(log.Fields{
					"address": address,
				}).Info("Added to queue successfully")
				resp := claimResponse{Message: fmt.Sprintf("Added %s to the queue", address)}
				renderJSON(w, resp, http.StatusOK)
			default:
				log.Warn("Max queue capacity reached")
				renderJSON(w, claimResponse{Message: "Faucet queue is too long, please try again later"}, http.StatusServiceUnavailable)
			}
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		var txHash common.Hash
		var err error
		if len(s.cfg.contractAddress) == 0 {
			txHash, err = s.Transfer(ctx, address, chain.EtherToWei(int64(s.cfg.ethAmount)), big.NewInt(int64(s.cfg.gasPrice)))
		} else {
			ABI := `[
				{
					"inputs": [
						{
							"internalType": "address",
							"name": "_owner",
							"type": "address"
						},
						{
							"internalType": "address",
							"name": "_usdtAddress",
							"type": "address"
						},
						{
							"internalType": "address",
							"name": "_usdcAddress",
							"type": "address"
						}
					],
					"stateMutability": "nonpayable",
					"type": "constructor"
				},
				{
					"inputs": [
						{
							"internalType": "address",
							"name": "_to",
							"type": "address"
						},
						{
							"internalType": "uint256",
							"name": "_ethAmount",
							"type": "uint256"
						},
						{
							"internalType": "uint256",
							"name": "_usdtAmount",
							"type": "uint256"
						},
						{
							"internalType": "uint256",
							"name": "_usdcAmount",
							"type": "uint256"
						}
					],
					"name": "multiTransfer",
					"outputs": [],
					"stateMutability": "nonpayable",
					"type": "function"
				},
				{
					"inputs": [],
					"name": "withdrawAllTokens",
					"outputs": [],
					"stateMutability": "nonpayable",
					"type": "function"
				},
				{
					"stateMutability": "payable",
					"type": "receive"
				}
			]`
			contract, err := abi.JSON(strings.NewReader(ABI))
			if err != nil {
				log.WithError(err).Error("Failed to import ABI")
			}

			to := common.HexToAddress(address)
			ethAmount := big.NewInt(int64(s.cfg.ethAmount))
			usdtAmount := big.NewInt(int64(s.cfg.usdtAmount))
			usdcAmount := big.NewInt(int64(s.cfg.usdcAmount))

			data, err := contract.Pack("multiTransfer", to, ethAmount, usdtAmount, usdcAmount)
			if err != nil {
				log.WithError(err).Error("Failed to encode ABI")
			}
			txHash, err = s.MultiTransfer(ctx, s.cfg.contractAddress, data, big.NewInt(int64(s.cfg.gasPrice)))
		}
		s.mutex.Unlock()
		if err != nil {
			log.WithError(err).Error("Failed to send transaction")
			renderJSON(w, claimResponse{Message: err.Error()}, http.StatusInternalServerError)
			return
		}

		log.WithFields(log.Fields{
			"txHash":  txHash,
			"address": address,
		}).Info("Funded directly successfully")
		resp := claimResponse{Message: fmt.Sprintf("TransactionHash:%s", txHash)}
		renderJSON(w, resp, http.StatusOK)
	}
}

func (s *Server) handleInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "GET" {
			http.NotFound(w, r)
			return
		}
		renderJSON(w, infoResponse{
			NetworkName:           s.cfg.networkName,
			FaucetEOAAddress:      s.Sender().String(),
			FaucetContractAddress: s.cfg.contractAddress,
			ETHAmount:             strconv.Itoa(s.cfg.ethAmount),
			USDTAmount:            strconv.Itoa(s.cfg.usdtAmount),
			USDCAmount:            strconv.Itoa(s.cfg.usdcAmount),
			Interval:              strconv.Itoa(s.cfg.interval),
		}, http.StatusOK)
	}
}
