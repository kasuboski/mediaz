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

type SubtitleFiles struct {
	ID           int32 `sql:"primary_key"`
	MovieId      int32
	MovieFileId  int32
	RelativePath string
	Extension    string
	Added        time.Time
	LastUpdated  time.Time
	Language     int32
	LanguageTags *string
}
