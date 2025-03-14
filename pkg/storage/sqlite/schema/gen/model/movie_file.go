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

type MovieFile struct {
	ID               int32 `sql:"primary_key"`
	Quality          string
	Size             int64
	DateAdded        time.Time
	SceneName        *string
	MediaInfo        *string
	ReleaseGroup     *string
	RelativePath     *string
	Edition          *string
	Languages        string
	IndexerFlags     int32
	OriginalFilePath *string
}
