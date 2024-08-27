//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package table

import (
	"github.com/go-jet/jet/v2/sqlite"
)

var NotificationStatus = newNotificationStatusTable("", "NotificationStatus", "")

type notificationStatusTable struct {
	sqlite.Table

	// Columns
	ID                sqlite.ColumnInteger
	ProviderId        sqlite.ColumnInteger
	InitialFailure    sqlite.ColumnTimestamp
	MostRecentFailure sqlite.ColumnTimestamp
	EscalationLevel   sqlite.ColumnInteger
	DisabledTill      sqlite.ColumnTimestamp

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type NotificationStatusTable struct {
	notificationStatusTable

	EXCLUDED notificationStatusTable
}

// AS creates new NotificationStatusTable with assigned alias
func (a NotificationStatusTable) AS(alias string) *NotificationStatusTable {
	return newNotificationStatusTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new NotificationStatusTable with assigned schema name
func (a NotificationStatusTable) FromSchema(schemaName string) *NotificationStatusTable {
	return newNotificationStatusTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new NotificationStatusTable with assigned table prefix
func (a NotificationStatusTable) WithPrefix(prefix string) *NotificationStatusTable {
	return newNotificationStatusTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new NotificationStatusTable with assigned table suffix
func (a NotificationStatusTable) WithSuffix(suffix string) *NotificationStatusTable {
	return newNotificationStatusTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newNotificationStatusTable(schemaName, tableName, alias string) *NotificationStatusTable {
	return &NotificationStatusTable{
		notificationStatusTable: newNotificationStatusTableImpl(schemaName, tableName, alias),
		EXCLUDED:                newNotificationStatusTableImpl("", "excluded", ""),
	}
}

func newNotificationStatusTableImpl(schemaName, tableName, alias string) notificationStatusTable {
	var (
		IDColumn                = sqlite.IntegerColumn("Id")
		ProviderIdColumn        = sqlite.IntegerColumn("ProviderId")
		InitialFailureColumn    = sqlite.TimestampColumn("InitialFailure")
		MostRecentFailureColumn = sqlite.TimestampColumn("MostRecentFailure")
		EscalationLevelColumn   = sqlite.IntegerColumn("EscalationLevel")
		DisabledTillColumn      = sqlite.TimestampColumn("DisabledTill")
		allColumns              = sqlite.ColumnList{IDColumn, ProviderIdColumn, InitialFailureColumn, MostRecentFailureColumn, EscalationLevelColumn, DisabledTillColumn}
		mutableColumns          = sqlite.ColumnList{ProviderIdColumn, InitialFailureColumn, MostRecentFailureColumn, EscalationLevelColumn, DisabledTillColumn}
	)

	return notificationStatusTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:                IDColumn,
		ProviderId:        ProviderIdColumn,
		InitialFailure:    InitialFailureColumn,
		MostRecentFailure: MostRecentFailureColumn,
		EscalationLevel:   EscalationLevelColumn,
		DisabledTill:      DisabledTillColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
