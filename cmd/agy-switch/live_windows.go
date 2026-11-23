package main

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// On Windows, agy keeps its login in the Credential Manager as a generic
// credential named "gemini:antigravity".
const credentialStoreName = "Windows Credential Manager"

func readLiveToken() ([]byte, error) {
	secret, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		return []byte(secret), nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("could not read the Windows credential: %w", err)
	}
	return readTokenFile()
}

func writeLiveToken(data []byte) error {
	if err := keyring.Set(keyringService, keyringUser, string(data)); err != nil {
		return fmt.Errorf("could not write the Windows credential: %w", err)
	}
	return nil
}

func clearLiveToken() error {
	err := keyring.Delete(keyringService, keyringUser)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("could not delete the Windows credential: %w", err)
	}
	return removeIfExists(liveTokenFile())
}
