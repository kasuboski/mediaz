//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

type QualityProfiles struct {
	ID                int32 `sql:"primary_key"`
	Name              string
	Cutoff            int32
	Items             string
	Language          *int32
	FormatItems       string
	UpgradeAllowed    *int32
	MinFormatScore    int32
	CutoffFormatScore int32
}
