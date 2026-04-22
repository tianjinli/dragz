package setting

import (
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/spf13/viper"
	"github.com/tianjinli/dragz/pkg/appkit"
	"github.com/tianjinli/dragz/pkg/config"
	"go.uber.org/zap"
)

type settingManager[T any] struct {
	logger *zap.Logger
	viper  *viper.Viper
	mu     sync.RWMutex
	events []appkit.SettingEvent[T]
	// atomic snapshot of the current settings instance (of user-provided type)
	snapshot atomic.Pointer[T] // stores: any (pointer to struct)
	// prototype to derive new instances (type only; never mutated)
	protoType reflect.Type
}

// NewSettingManager creates a SettingManager using either a file path or a DataID together with the provided metadata.
func NewSettingManager[T any](logger *zap.Logger, filename string, metadata any) (appkit.SettingManager[T], error) {
	return (&settingManager[T]{
		viper:  viper.New(),
		logger: logger,
	}).loadAndWatch(filename, metadata)
}

func (s *settingManager[T]) GetSnapshot() *T {
	return s.snapshot.Load()
}

func (s *settingManager[T]) AddEvent(event appkit.SettingEvent[T]) {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
}

func (s *settingManager[T]) RemoveEvent(event appkit.SettingEvent[T]) {
	s.mu.Lock()
	filtered := make([]appkit.SettingEvent[T], 0, len(s.events))
	for _, e := range s.events {
		if e != event {
			filtered = append(filtered, e)
		}
	}
	s.events = filtered
	s.mu.Unlock()
}

// reloadSetting reloads from viper and swaps the atomic snapshot.
// It also fires OnChanged on registered events without holding locks during callbacks.
func (s *settingManager[T]) reloadSetting(filename string) {
	// Optional: merge global viper if needed
	if err := config.ReverseMergeViper(s.viper, viper.GetViper()); err != nil {
		s.logger.Warn("viper merge failed", zap.String("file", filename), zap.Error(err))
	}

	// Create a new instance of the provided type
	newInst := new(T)
	if err := s.viper.Unmarshal(newInst, config.YamlTagDecoder); err != nil {
		s.logger.Warn("reload setting failed", zap.String("file", filename), zap.Error(err))
		return
	}

	s.snapshot.Store(newInst)
	s.logger.Info("reload setting success", zap.String("file", filename))

	// Copy events under read lock, then invoke callbacks without holding the lock
	s.mu.RLock()
	listeners := make([]appkit.SettingEvent[T], len(s.events))
	copy(listeners, s.events)
	s.mu.RUnlock()

	for _, event := range listeners {
		// Best effort; log errors inside event implementations if needed
		event.OnChanged(newInst)
	}
}
