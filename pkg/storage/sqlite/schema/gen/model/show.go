//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

import (
	"time"
)

type Show struct {
	ID               int32 `sql:"primary_key"`
	Monitored        int32
	QualityProfileID int32
	Added            *time.Time
	ShowMetadata     *int32
	LastSearchTime   *time.Time
}
