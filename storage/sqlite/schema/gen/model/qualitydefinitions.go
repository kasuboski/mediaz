//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

type QualityDefinitions struct {
	ID            int32 `sql:"primary_key"`
	Quality       int32
	Title         string
	MinSize       *float64
	MaxSize       *float64
	PreferredSize *float64
}
