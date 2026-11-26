package app

import (
	"reflect"
	"testing"

	metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"
)

func TestExplorerSelectionSelectDatabaseClearsPreviousObjects(t *testing.T) {
	state := explorerSelection{
		databases: []string{"default", "analytics"},
		selectedDatabase: "default",
		selectedTable: "events",
		selectedObject: "default.events",
		tables: []metadataapp.Object{{Name: "events"}},
		views: []metadataapp.Object{{Name: "events_view"}},
		columns: []metadataapp.Column{{Name: "id"}},
		tableDetails: metadataapp.TableDetails{CreateTableQuery: "CREATE TABLE events"},
	}

	state.SelectDatabase("analytics")

	if state.selectedDatabase != "analytics" {
		t.Fatalf("selectedDatabase = %q, want analytics", state.selectedDatabase)
	}
	if state.selectedTable != "" || state.selectedObject != "" {
		t.Fatalf("table/object selection was not cleared: table=%q object=%q", state.selectedTable, state.selectedObject)
	}
	if state.tables != nil || state.views != nil || state.columns != nil {
		t.Fatalf("previous database data was not cleared")
	}
	if !reflect.DeepEqual(state.tableDetails, metadataapp.TableDetails{}) {
		t.Fatalf("tableDetails = %#v, want zero value", state.tableDetails)
	}
}

func TestExplorerSelectionResetConnectionClearsAllNavigationState(t *testing.T) {
	state := explorerSelection{
		databases: []string{"analytics"},
		selectedDatabase: "analytics",
		selectedTable: "events",
		selectedObject: "analytics.events",
		tables: []metadataapp.Object{{Name: "events"}},
		views: []metadataapp.Object{{Name: "events_view"}},
		columns: []metadataapp.Column{{Name: "id"}},
		tableDetails: metadataapp.TableDetails{CreateTableQuery: "CREATE TABLE events"},
	}

	state.ResetConnection()

	if state.databases != nil || state.selectedDatabase != "" || state.selectedTable != "" || state.selectedObject != "" {
		t.Fatalf("connection navigation state was not reset: %#v", state)
	}
	if state.tables != nil || state.views != nil || state.columns != nil {
		t.Fatalf("connection object data was not reset")
	}
	if !reflect.DeepEqual(state.tableDetails, metadataapp.TableDetails{}) {
		t.Fatalf("tableDetails = %#v, want zero value", state.tableDetails)
	}
}

func TestExplorerSelectionSetSchemaAndClearSchema(t *testing.T) {
	state := explorerSelection{}
	columns := []metadataapp.Column{{Name: "id"}}
	details := metadataapp.TableDetails{CreateTableQuery: "CREATE TABLE events"}

	state.SetSchema(columns, details)
	if !reflect.DeepEqual(state.columns, columns) || !reflect.DeepEqual(state.tableDetails, details) {
		t.Fatalf("SetSchema did not retain schema: columns=%#v details=%#v", state.columns, state.tableDetails)
	}

	state.ClearSchema()
	if state.columns != nil || !reflect.DeepEqual(state.tableDetails, metadataapp.TableDetails{}) {
		t.Fatalf("ClearSchema did not clear schema: columns=%#v details=%#v", state.columns, state.tableDetails)
	}
}

func TestExplorerSelectionSelectTableClearsPreviousSchema(t *testing.T) {
	state := explorerSelection{
		selectedDatabase: "analytics",
		selectedTable: "old_events",
		selectedObject: "analytics.old_events",
		columns: []metadataapp.Column{{Name: "old_id"}},
		tableDetails: metadataapp.TableDetails{CreateTableQuery: "CREATE TABLE old_events"},
	}

	state.SelectTable("new_events")

	if state.selectedTable != "new_events" {
		t.Fatalf("selectedTable = %q, want new_events", state.selectedTable)
	}
	if state.columns != nil || !reflect.DeepEqual(state.tableDetails, metadataapp.TableDetails{}) {
		t.Fatalf("previous table schema was not cleared: columns=%#v details=%#v", state.columns, state.tableDetails)
	}
	if state.selectedDatabase != "analytics" {
		t.Fatalf("database selection changed unexpectedly: %q", state.selectedDatabase)
	}
}
