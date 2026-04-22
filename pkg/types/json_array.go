package types

import (
	"database/sql/driver"
	"encoding/json"
)

type JsonArray[T any] []T

func (a JsonArray[T]) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

func (a *JsonArray[T]) Scan(input any) error {
	if input == nil {
		*a = nil
		return nil
	}
	bytes := input.([]byte)
	return json.Unmarshal(bytes, a)
}
