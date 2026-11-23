package main

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/zalando/go-keyring"
)

// On Linux, agy keeps its login in the Secret Service keyring (GNOME Keyring,
// KWallet, ...) and falls back to the token file only when there is none.
const credentialStoreName = "system keyring"

const secretsBusName = "org.freedesktop.secrets"

var keyringAvailable = sync.OnceValue(func() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	bus := conn.BusObject()
	var has bool
	if bus.Call("org.freedesktop.DBus.NameHasOwner", 0, secretsBusName).Store(&has) == nil && has {
		return true
	}
	var names []string
	if bus.Call("org.freedesktop.DBus.ListActivatableNames", 0).Store(&names) != nil {
		return false
	}
	return slices.Contains(names, secretsBusName)
})

func readLiveToken() ([]byte, error) {
	if keyringAvailable() {
		secret, err := keyring.Get(keyringService, keyringUser)
		if err == nil {
			return []byte(secret), nil
		}
		if !errors.Is(err, keyring.ErrNotFound) {
			return nil, fmt.Errorf("could not read the system keyring (is it locked?): %w", err)
		}
	}
	return readTokenFile()
}

func writeLiveToken(data []byte) error {
	if !keyringAvailable() {
		return atomicWrite(liveTokenFile(), data)
	}
	if err := keyring.Set(keyringService, keyringUser, string(data)); err != nil {
		return fmt.Errorf("could not write the system keyring: %w", err)
	}
	return nil
}

func clearLiveToken() error {
	if keyringAvailable() {
		for range 100 {
			err := keyring.Delete(keyringService, keyringUser)
			if errors.Is(err, keyring.ErrNotFound) {
				break
			}
			if err != nil {
				return fmt.Errorf("could not delete from the system keyring: %w", err)
			}
		}
	}
	return removeIfExists(liveTokenFile())
}
