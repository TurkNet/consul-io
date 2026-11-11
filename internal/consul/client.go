package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
)

func getClient(consulAddr, token string) (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = consulAddr

	if token != "" {
		config.Token = token
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("Error creating Consul client: %v", err)
	}

	if _, err = client.Agent().Self(); err != nil {
		return nil, fmt.Errorf("Failed to authenticate with Consul using the provided token: %v", err)
	}

	return client, nil
}

// Store, syncstate.KVStore arayüzünü Consul üzerinden uygular.
type Store struct {
	client     *api.Client
	retryLimit int
	rateLimit  time.Duration
	ticker     *time.Ticker
}

// NewStore, verilen adrese bağlanan bir Consul Store döner.
func NewStore(consulAddr, token string, retryLimit, rateLimit int, ticker *time.Ticker) (*Store, error) {
	client, err := getClient(consulAddr, token)
	if err != nil {
		return nil, err
	}
	return &Store{
		client:     client,
		retryLimit: retryLimit,
		rateLimit:  time.Duration(rateLimit) * time.Millisecond,
		ticker:     ticker,
	}, nil
}

// Get, anahtarın değerini Consul'dan okur.
func (s *Store) Get(key string) ([]byte, bool, error) {
	var lastErr error
	for i := 0; i < s.retryLimit; i++ {
		if s.ticker != nil {
			<-s.ticker.C
		}
		kvPair, _, err := s.client.KV().Get(key, nil)
		if err == nil {
			if kvPair == nil {
				return nil, false, nil
			}
			return kvPair.Value, true, nil
		}
		lastErr = err
		time.Sleep(s.rateLimit)
	}
	return nil, false, fmt.Errorf("failed to get KV from Consul: %v", lastErr)
}

// Put, anahtarı Consul'a yazar.
func (s *Store) Put(key string, value []byte) error {
	var lastErr error
	kv := s.client.KV()
	for i := 0; i < s.retryLimit; i++ {
		if s.ticker != nil {
			<-s.ticker.C
		}
		if _, err := kv.Put(&api.KVPair{Key: key, Value: value}, nil); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(s.rateLimit)
	}
	return fmt.Errorf("failed to put KV to Consul: %v", lastErr)
}

// Delete, anahtarı Consul'dan siler.
func (s *Store) Delete(key string) error {
	var lastErr error
	for i := 0; i < s.retryLimit; i++ {
		if s.ticker != nil {
			<-s.ticker.C
		}
		if _, err := s.client.KV().Delete(key, nil); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(s.rateLimit)
	}
	return fmt.Errorf("failed to delete KV from Consul: %v", lastErr)
}
