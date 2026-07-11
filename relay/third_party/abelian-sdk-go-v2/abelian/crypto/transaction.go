package crypto

import (
	api "github.com/pqabelian/abec/sdkapi/v2"
	"github.com/pqabelian/abec/wire"
)

func GenerateTransferTransactionByRootSeeds(transactionRequest []byte, serializedCryptoSeeds [][]byte) ([]byte, []byte, error) {
	var err error
	cryptoRootSeeds := make([]*CryptoSeeds, len(serializedCryptoSeeds))
	for i := 0; i < len(serializedCryptoSeeds); i++ {
		cryptoRootSeeds[i], err = deserializeSeed(serializedCryptoSeeds[i])
		if err != nil {
			return nil, nil, err
		}
	}
	apiCryptoSeeds := make([]*api.CryptoRootSeed, len(cryptoRootSeeds))
	for i := 0; i < len(cryptoRootSeeds); i++ {
		currentRootSeed := cryptoRootSeeds[i]
		apiCryptoSeeds[i] = api.NewRootSeed(
			currentRootSeed.cryptoScheme,
			currentRootSeed.privacyLevel,
			currentRootSeed.coinSpendKeySeed,
			currentRootSeed.coinSerialNumberKeySeed,
			currentRootSeed.coinValueKeySeed,
			currentRootSeed.coinDetectorKey,
		)
	}
	serializedTxFull, txId, err := api.CreateTransferTxByRootSeed(wire.TxVersion, transactionRequest, apiCryptoSeeds)
	if err != nil {
		return nil, nil, err
	}
	return serializedTxFull, txId[:], nil
}
