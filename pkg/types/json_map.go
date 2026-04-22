package types

import (
	"database/sql/driver"
	"encoding/json"
)

type JsonMap[K comparable, V any] map[K]V

func (m JsonMap[K, V]) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(m) // <=> bytes, err := json.Marshal(&m)
	return string(bytes), err
}

func (m *JsonMap[K, V]) Scan(input any) error {
	if input == nil {
		*m = nil
		return nil
	}
	return json.Unmarshal(input.([]byte), m)
}
