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

type MovieMetadata struct {
	ID                 int32 `sql:"primary_key"`
	TmdbId             int32
	ImdbId             *string
	Images             string
	Genres             *string
	Title              string
	SortTitle          *string
	CleanTitle         *string
	OriginalTitle      *string
	CleanOriginalTitle *string
	OriginalLanguage   int32
	Status             int32
	LastInfoSync       *time.Time
	Runtime            int32
	InCinemas          *time.Time
	PhysicalRelease    *time.Time
	DigitalRelease     *time.Time
	Year               *int32
	SecondaryYear      *int32
	Ratings            *string
	Recommendations    string
	Certification      *string
	YouTubeTrailerId   *string
	Studio             *string
	Overview           *string
	Website            *string
	Popularity         *float64
	CollectionTmdbId   *int32
	CollectionTitle    *string
}