package cache_utils

import (
	"context"
	"crypto/tls"
	"sync"

	"github.com/valkey-io/valkey-go"

	"databasus-backend/internal/config"
)

var (
	once         sync.Once
	valkeyClient valkey.Client
)

func getCache() valkey.Client {
	once.Do(func() {
		env := config.GetEnv()

		options := valkey.ClientOption{
			InitAddress: []string{env.ValkeyHost + ":" + env.ValkeyPort},
			Password:    env.ValkeyPassword,
			Username:    env.ValkeyUsername,
		}

		if env.ValkeyIsSsl {
			options.TLSConfig = &tls.Config{
				ServerName: env.ValkeyHost,
			}
		}

		client, err := valkey.NewClient(options)
		if err != nil {
			panic(err)
		}

		valkeyClient = client
	})

	return valkeyClient
}

func GetValkeyClient() valkey.Client {
	return getCache()
}

func TestCacheConnection() {
	// Get Valkey client from cache package
	client := getCache()

	// Create a simple test cache util for strings
	cacheUtil := NewCacheUtil[string](client, "test:")

	// Test data
	testKey := "connection_test"
	testValue := "valkey_is_working"

	// Test Set operation
	cacheUtil.Set(testKey, &testValue)

	// Test Get operation
	retrievedValue := cacheUtil.Get(testKey)

	// Verify the value was retrieved correctly
	if retrievedValue == nil {
		panic("Cache test failed: could not retrieve cached value")
	}

	if *retrievedValue != testValue {
		panic("Cache test failed: retrieved value does not match expected")
	}

	// Clean up - remove test key
	cacheUtil.Invalidate(testKey)

	// Verify cleanup worked
	cleanupCheck := cacheUtil.Get(testKey)
	if cleanupCheck != nil {
		panic("Cache test failed: test key was not properly invalidated")
	}
}

func ClearAllCache() error {
	pattern := "*"
	cursor := uint64(0)
	batchSize := int64(100)

	cacheUtil := NewCacheUtil[string](getCache(), "")

	for {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultQueueTimeout)

		result := cacheUtil.client.Do(
			ctx,
			cacheUtil.client.B().Scan().Cursor(cursor).Match(pattern).Count(batchSize).Build(),
		)
		cancel()

		if result.Error() != nil {
			return result.Error()
		}

		scanResult, err := result.AsScanEntry()
		if err != nil {
			return err
		}

		if len(scanResult.Elements) > 0 {
			delCtx, delCancel := context.WithTimeout(context.Background(), cacheUtil.timeout)
			cacheUtil.client.Do(
				delCtx,
				cacheUtil.client.B().Del().Key(scanResult.Elements...).Build(),
			)
			delCancel()
		}

		cursor = scanResult.Cursor
		if cursor == 0 {
			break
		}
	}

	return nil
}
