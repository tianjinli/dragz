package entity

import (
	"time"
)

type Demo struct {
	Package string `gorm:"size:32;not null;uniqueIndex:uix_atlas_demo_package_struct;comment:Package Name"`
	Struct  string `gorm:"size:64;not null;uniqueIndex:uix_atlas_demo_package_struct;comment:Struct Name"`

	Table    string `gorm:"size:128;primaryKey;comment:Table Name"`
	Checksum string `gorm:"comment:Checksum of file"`

	CreatedAt time.Time `gorm:"comment:Created Time"`
	UpdatedAt time.Time `gorm:"comment:Updated Time"`
}

func (*Demo) TableName() string {
	return "atlas_demo"
}
