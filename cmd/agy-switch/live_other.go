//go:build !linux && !windows

package main

// On other systems (macOS) only the token file is supported. A login that agy
// keeps in the macOS Keychain isn't detected.
const credentialStoreName = "token file"

func readLiveToken() ([]byte, error) { return readTokenFile() }

func writeLiveToken(data []byte) error { return atomicWrite(liveTokenFile(), data) }

func clearLiveToken() error { return removeIfExists(liveTokenFile()) }
