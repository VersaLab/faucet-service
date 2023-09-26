package chain

import (
	"context"
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type TxBuilder interface {
	Sender() common.Address
	Transfer(ctx context.Context, to string, value *big.Int, gasprice *big.Int) (common.Hash, error)
	MultiTransfer(ctx context.Context, to string, data []byte, gasprice *big.Int) (common.Hash, error)
}

type TxBuild struct {
	client      bind.ContractTransactor
	privateKey  *ecdsa.PrivateKey
	signer      types.Signer
	fromAddress common.Address
}

func NewTxBuilder(provider string, privateKey *ecdsa.PrivateKey, chainID *big.Int) (TxBuilder, error) {
	client, err := ethclient.Dial(provider)
	if err != nil {
		return nil, err
	}

	if chainID == nil {
		chainID, err = client.ChainID(context.Background())
		if err != nil {
			return nil, err
		}
	}

	return &TxBuild{
		client:      client,
		privateKey:  privateKey,
		signer:      types.NewEIP155Signer(chainID),
		fromAddress: crypto.PubkeyToAddress(privateKey.PublicKey),
	}, nil
}

func (b *TxBuild) Sender() common.Address {
	return b.fromAddress
}

func (b *TxBuild) Transfer(ctx context.Context, to string, value *big.Int, gasprice *big.Int) (common.Hash, error) {
	nonce, err := b.client.PendingNonceAt(ctx, b.Sender())
	if err != nil {
		return common.Hash{}, err
	}

	gasLimit := uint64(50000)
	var gasPrice *big.Int
	if gasprice.Cmp(big.NewInt(0)) == 0 {
		gasPrice, err = b.client.SuggestGasPrice(ctx)
		if err != nil {
			return common.Hash{}, err
		}
	} else {
		gasPrice = gasprice
	}

	toAddress := common.HexToAddress(to)
	unsignedTx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &toAddress,
		Value:    value,
		Gas:      gasLimit,
		GasPrice: gasPrice,
	})

	signedTx, err := types.SignTx(unsignedTx, b.signer, b.privateKey)
	if err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), b.client.SendTransaction(ctx, signedTx)
}

func (b *TxBuild) MultiTransfer(ctx context.Context, to string, data []byte, gasprice *big.Int) (common.Hash, error) {
	nonce, err := b.client.PendingNonceAt(ctx, b.Sender())
	if err != nil {
		return common.Hash{}, err
	}

	toAddress := common.HexToAddress(to)
	gasLimit := uint64(500000)
	var gasPrice *big.Int
	if gasprice.Cmp(big.NewInt(0)) == 0 {
		gasPrice, err = b.client.SuggestGasPrice(ctx)
		if err != nil {
			return common.Hash{}, err
		}
	} else {
		gasPrice = gasprice
	}

	unsignedTx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &toAddress,
		Value:    big.NewInt(0),
		Data:     data,
		Gas:      gasLimit,
		GasPrice: gasPrice,
	})

	signedTx, err := types.SignTx(unsignedTx, b.signer, b.privateKey)
	if err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), b.client.SendTransaction(ctx, signedTx)
}
