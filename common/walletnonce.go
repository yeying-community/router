package common

import (
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/random"
)

type walletNonceValue struct {
	Nonce    string
	Message  string
	ExpireAt time.Time
}

// simple in-memory nonce store, valid for 10 minutes
var (
	walletNonceMutex sync.Mutex
	walletNonceMap   = make(map[string]walletNonceValue) // key: lower-case address
	walletNonceTTL   = 10 * time.Minute
)

// GenerateWalletNonce creates a nonce & message and stores them for later verification
func GenerateWalletNonce(address, messagePrefix, chainId string) (nonce string, message string) {
	addr := strings.ToLower(address)
	nonce = random.GetUUID()
	now := time.Now()
	message = messagePrefix + "\n" +
		"Nonce: " + nonce + "\n" +
		"Address: " + address + "\n" +
		"Issued At: " + now.UTC().Format(time.RFC3339)
	if chainId != "" {
		message += "\nChainId: " + chainId
	}

	walletNonceMutex.Lock()
	defer walletNonceMutex.Unlock()
	walletNonceMap[addr] = walletNonceValue{
		Nonce:    nonce,
		Message:  message,
		ExpireAt: now.Add(getWalletNonceTTL()),
	}
	cleanupWalletNonces()
	return
}

func getWalletNonceTTL() time.Duration {
	if config.NonceTTLMinutes <= 0 {
		return walletNonceTTL
	}
	return time.Duration(config.NonceTTLMinutes) * time.Minute
}

// GetWalletNonce returns stored nonce entry if valid
func GetWalletNonce(address string) (walletNonceValue, bool) {
	walletNonceMutex.Lock()
	defer walletNonceMutex.Unlock()
	entry, ok := walletNonceMap[strings.ToLower(address)]
	if !ok || time.Now().After(entry.ExpireAt) {
		return walletNonceValue{}, false
	}
	return entry, true
}

// ConsumeWalletNonce removes a nonce (used after successful auth)
func ConsumeWalletNonce(address string) {
	walletNonceMutex.Lock()
	defer walletNonceMutex.Unlock()
	delete(walletNonceMap, strings.ToLower(address))
}

func cleanupWalletNonces() {
	now := time.Now()
	for addr, entry := range walletNonceMap {
		if now.After(entry.ExpireAt) {
			delete(walletNonceMap, addr)
		}
	}
}
