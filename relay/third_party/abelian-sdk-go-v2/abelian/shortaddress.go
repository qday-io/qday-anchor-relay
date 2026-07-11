package abelian

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/pqabelian/abelian-sdk-go-v2/abelian/crypto"
)

const (
	SHORT_ABEL_ADDRESS_LENGTH    = 66
	SHORT_ABEL_ADDRESS_V2_LENGTH = 68
)

// ShortAbelAddress define short address
type ShortAbelAddress struct {
	data []byte
}

func (address *ShortAbelAddress) Data() []byte {
	return address.data
}

func (address *ShortAbelAddress) Validate() error {
	if len(address.data) != SHORT_ABEL_ADDRESS_LENGTH && len(address.data) != SHORT_ABEL_ADDRESS_V2_LENGTH {
		return fmt.Errorf("short abel address data length is not %d or %d", SHORT_ABEL_ADDRESS_LENGTH, SHORT_ABEL_ADDRESS_V2_LENGTH)
	}

	if address.data[0] != 0xab {
		return fmt.Errorf("short abel address data is not prefixed with 0xab")
	}

	chainID := address.data[1] - 0xe0
	if chainID < 0 || chainID > 15 {
		return fmt.Errorf("short abel address chain id is not in range [0, 15]")
	}

	return nil
}

// NewShortAbelAddress
func NewShortAbelAddress(chainID NetworkID, fingerprint []byte, abelAddressHash []byte) (*ShortAbelAddress, error) {
	saData := make([]byte, 0, 2+len(fingerprint)+len(abelAddressHash))
	saData = append(saData, 0xab, 0xe1+byte(chainID))
	saData = append(saData, fingerprint...)
	saData = append(saData, abelAddressHash...)

	if len(saData) != SHORT_ABEL_ADDRESS_LENGTH {
		return nil, crypto.ErrInvalidAddress
	}

	return &ShortAbelAddress{data: saData}, nil
}

type MetaData struct {
	Version      uint8
	NetID        NetworkID
	CryptoScheme crypto.CryptoScheme
	PrivacyLevel crypto.PrivacyLevel
}

func (metadata *MetaData) Validate() error {
	if metadata.Version < 0 || metadata.Version > 15 {
		return errors.New("invalid version for short address")
	}
	if metadata.NetID < 0 || metadata.NetID > 15 {
		return fmt.Errorf("invalid abelian network id")
	}
	if metadata.CryptoScheme < 0 || metadata.CryptoScheme > 4 {
		return fmt.Errorf("invalid crypto scheme")
	}
	if metadata.PrivacyLevel < 0 || metadata.PrivacyLevel > 4 {
		return fmt.Errorf("invalid privacy level")
	}
	return nil
}
func (metadata *MetaData) Bytes() ([]byte, error) {
	if err := metadata.Validate(); err != nil {
		return nil, err
	}
	firstBytes := uint8(metadata.NetID)&0x0F | ((metadata.Version & 0x0F) << 4)
	secondBytes := 0x00<<4 | ((uint8(metadata.CryptoScheme) & 0x03) << 2) | ((uint8(metadata.PrivacyLevel) & 0x03) << 0)
	return []byte{firstBytes, secondBytes}, nil
}
func NewShortAbelAddressV2(metadata *MetaData, fingerprint []byte, cryptoAddressHash []byte) (*ShortAbelAddress, error) {
	saData := make([]byte, 0, 2+2+len(fingerprint)+len(cryptoAddressHash))

	saData = append(saData, 0xab, 0xe0)

	bytesForMetaData, err := metadata.Bytes()
	if err != nil {
		return nil, crypto.ErrInvalidAddress
	}
	saData = append(saData, bytesForMetaData...) // metadata

	saData = append(saData, fingerprint...)
	saData = append(saData, cryptoAddressHash...)

	if len(saData) != SHORT_ABEL_ADDRESS_V2_LENGTH {
		return nil, crypto.ErrInvalidAddress
	}

	return &ShortAbelAddress{data: saData}, nil
}

func GetShortAbelAddressFromAbelAddress(address *AbelAddress) (*ShortAbelAddress, error) {
	hash := sha256.Sum256(address.data)

	if address.cryptoAddress.GetCryptoScheme() == crypto.CryptoSchemePQRingCT {
		return NewShortAbelAddress(
			address.GetNetID(),
			address.GetCryptoAddress().GetCoinAddress().Fingerprint(),
			hash[:],
		)
	} else if address.cryptoAddress.GetCryptoScheme() == crypto.CryptoSchemePQRingCTX {
		metadata := &MetaData{
			Version:      1,
			NetID:        address.GetNetID(),
			CryptoScheme: address.GetCryptoAddress().GetCryptoScheme(),
			PrivacyLevel: address.GetCryptoAddress().GetPrivacyLevel(),
		}
		return NewShortAbelAddressV2(metadata, address.GetCryptoAddress().GetCoinAddress().Fingerprint(),
			hash[:])
	}
	return nil, errors.New("unsupported abel address")

}
