//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package model

type QualityDefinition struct {
	ID            int32 `sql:"primary_key"`
	QualityID     *int32
	Name          string
	PreferredSize float32
	MinSize       float32
	MaxSize       float32
	MediaType     string
}
