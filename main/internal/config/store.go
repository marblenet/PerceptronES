package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu      sync.RWMutex
	path    string
	current Config
}

func LoadOrCreate(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("configuration path must not be empty")
	}
	cfg, err := Load(path)
	if err == nil {
		return cfg, nil
	}
	if !os.IsNotExist(rootCause(err)) {
		return Config{}, err
	}
	cfg = Default()
	if err := writeFileAtomically(path, cfg); err != nil {
		return Config{}, fmt.Errorf("create default configuration %q: %w", path, err)
	}
	return cfg, nil
}

func NewStore(path string, initial Config) (*Store, error) {
	if path == "" {
		return nil, errors.New("configuration path must not be empty")
	}
	if err := initial.Validate(); err != nil {
		return nil, fmt.Errorf("invalid initial configuration: %w", err)
	}
	return &Store{path: path, current: clone(initial)}, nil
}

func (store *Store) Path() string {
	return store.path
}

func (store *Store) Current() Config {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return clone(store.current)
}

func (store *Store) Reload() (bool, error) {
	return store.ReloadValidated(nil)
}

func (store *Store) ReloadValidated(validate func(Config) error) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	next, err := Load(store.path)
	if err != nil {
		return false, err
	}
	if validate != nil {
		if err := validate(next); err != nil {
			return false, fmt.Errorf("validate reloaded configuration: %w", err)
		}
	}
	encodedCurrent, err := json.Marshal(store.current)
	if err != nil {
		return false, fmt.Errorf("encode current configuration: %w", err)
	}
	encodedNext, err := json.Marshal(next)
	if err != nil {
		return false, fmt.Errorf("encode loaded configuration: %w", err)
	}
	if bytes.Equal(encodedCurrent, encodedNext) {
		return false, nil
	}
	store.current = clone(next)
	return true, nil
}

func (store *Store) Update(next Config) error {
	if err := next.Validate(); err != nil {
		return fmt.Errorf("validate updated configuration: %w", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := writeFileAtomically(store.path, next); err != nil {
		return fmt.Errorf("persist updated configuration: %w", err)
	}
	store.current = clone(next)
	return nil
}

func LoadFrom(reader io.Reader) (Config, error) {
	cfg := Default()
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration: %w", err)
	}
	content = bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf})
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Config{}, errors.New("multiple JSON values are not allowed")
		}
		return Config{}, fmt.Errorf("decode configuration trailing data: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate configuration: %w", err)
	}
	return cfg, nil
}

func writeFileAtomically(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	directory := filepath.Dir(path)
	temp, err := ioutil.TempFile(directory, ".perceptron-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config in %q: %w", directory, err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		temp.Close()
		return fmt.Errorf("encode temporary config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config file %q: %w", path, err)
	}
	return nil
}

func clone(cfg Config) Config {
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return cfg
	}
	var result Config
	if err := json.Unmarshal(encoded, &result); err != nil {
		return cfg
	}
	return result
}

func rootCause(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}
