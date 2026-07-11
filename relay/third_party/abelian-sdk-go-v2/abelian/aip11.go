package abelian

import (
	aip11 "github.com/pqabelian/abelian-aip11-go"
	"github.com/pqabelian/abelian-aip11-go/wordlists"
)

func GenerateEntropySeed() ([]byte, error) {
	return aip11.SampleEntropySeed()
}

func EntropySeedToMnemonics(entropySeed []byte) ([]string, error) {
	return aip11.EntropySeedToMnemonic(entropySeed, wordlists.English)
}

func MnemonicsToEntropySeed(mnemonics []string) ([]byte, error) {
	return aip11.MnemonicToEntropySeed(mnemonics, wordlists.English)
}

func EntropySeedToCryptoSeed(entropySeed []byte) (
	SpendKeyRootSeed []byte,
	SerialNoKeyRootSeed []byte,
	DetectorRootKey []byte,
	ViewKeyRootSeed []byte,
	err error) {
	masterSeed, err := aip11.EntropySeedToMasterSeed(entropySeed, []byte{})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	accountRootSeeds, err := aip11.MasterSeedToAccountRootSeeds(masterSeed)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return accountRootSeeds[0], accountRootSeeds[1], accountRootSeeds[2], accountRootSeeds[3], nil
}

func MnemonicsToCryptoSeed(mnemonics []string) (SpendKeyRootSeed []byte,
	SerialNoKeyRootSeed []byte,
	DetectorRootKey []byte,
	ViewKeyRootSeed []byte,
	err error) {

	entropySeed, err := aip11.MnemonicToEntropySeed(mnemonics, wordlists.English)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	masterSeed, err := aip11.EntropySeedToMasterSeed(entropySeed, []byte{})
	if err != nil {
		return nil, nil, nil, nil, err
	}
	accountRootSeeds, err := aip11.MasterSeedToAccountRootSeeds(masterSeed)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return accountRootSeeds[0], accountRootSeeds[1], accountRootSeeds[2], accountRootSeeds[3], nil
}

func EntropySeedToPublicRandRootSeed(entropySeed []byte) ([]byte, error) {
	masterSeed, err := aip11.EntropySeedToMasterSeed(entropySeed, []byte{})
	if err != nil {
		return nil, err
	}
	return aip11.MasterSeedToAccountPublicRandRootSeed(masterSeed)

}

func MnemonicsToPublicRandRootSeed(mnemonics []string) ([]byte, error) {
	entropySeed, err := aip11.MnemonicToEntropySeed(mnemonics, wordlists.English)
	if err != nil {
		return nil, err
	}
	masterSeed, err := aip11.EntropySeedToMasterSeed(entropySeed, []byte{})
	if err != nil {
		return nil, err
	}
	return aip11.MasterSeedToAccountPublicRandRootSeed(masterSeed)
}
func SequenceNoToPublicRand(publicRandRootSeed []byte, sequenceNo uint32) ([]byte, error) {
	return aip11.DerivePublicRand(publicRandRootSeed, sequenceNo)
}
