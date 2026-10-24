package sqlite

import (
	"math/bits"
	"os"
	"path/filepath"
	"sync"

	"github.com/ncruces/go-sqlite3"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

var configureOnce sync.Once

// configureRuntime enables wazero's on-disk compilation cache before the first
// connection is opened. Without it every noot process recompiles the SQLite
// Wasm binary, which costs about a second per command. The cache lives in the
// user cache dir (%LocalAppData%\noot\wazero, ~/.cache/noot/wazero); when it
// can't be created noot still works, just slower.
func configureRuntime() {
	configureOnce.Do(func() {
		// Same defaults as go-sqlite3 uses when RuntimeConfig is nil.
		cfg := wazero.NewRuntimeConfig().WithCoreFeatures(api.CoreFeaturesV2)
		if bits.UintSize < 64 {
			cfg = cfg.WithMemoryLimitPages(512) // 32MB
		} else {
			cfg = cfg.WithMemoryLimitPages(4096) // 256MB
		}
		if dir, err := cacheDir(); err == nil {
			if cache, err := wazero.NewCompilationCacheWithDir(dir); err == nil {
				cfg = cfg.WithCompilationCache(cache)
			}
		}
		sqlite3.RuntimeConfig = cfg
	})
}

func cacheDir() (string, error) {
	if d := os.Getenv("NOOT_CACHE_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "noot", "wazero"), nil
}
