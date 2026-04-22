package appkit

type SettingEvent[T any] interface {
	OnChanged(*T)
}

type SettingManager[T any] interface {
	GetSnapshot() *T
	AddEvent(event SettingEvent[T])
	RemoveEvent(event SettingEvent[T])
}
