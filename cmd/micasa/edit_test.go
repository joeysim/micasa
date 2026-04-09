// Copyright 2026 Phillip Cloud
// Licensed under the Apache License, Version 2.0

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listJSON(t *testing.T, dbPath, entity string, includeDeleted bool) []map[string]any {
	t.Helper()
	args := []string{entity, "list", "--json"}
	if includeDeleted {
		args = append(args, "--deleted")
	}
	args = append(args, dbPath)
	out, err := executeCLI(args...)
	require.NoError(t, err)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rows))
	return rows
}

func firstID(t *testing.T, dbPath, entity string) string {
	t.Helper()
	args := []string{entity, "list", "--json", dbPath}
	if entity == "project-types" || entity == "maintenance-categories" {
		args = []string{"show", entity, "--json", dbPath}
	}
	out, err := executeCLI(args...)
	require.NoError(t, err)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rows))
	require.NotEmpty(t, rows, "expected at least one row for %s", entity)
	id, ok := rows[0]["id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, id)
	return id
}

func TestEditVendorLifecycleCLI(t *testing.T) {
	t.Parallel()
	dbPath := createTestDB(t)

	createOut, err := executeCLI(
		"vendors",
		"add",
		"--data",
		`{"name":"Acme Plumbing","email":"ops@acme.example"}`,
		dbPath,
	)
	require.NoError(t, err)
	vendorID := strings.TrimSpace(createOut)
	require.NotEmpty(t, vendorID)

	_, err = executeCLI(
		"vendors",
		"edit",
		vendorID,
		"--data",
		`{"name":"Acme Plumbing and Heating","phone":"5551239999"}`,
		dbPath,
	)
	require.NoError(t, err)

	_, err = executeCLI("vendors", "delete", vendorID, dbPath)
	require.NoError(t, err)

	deletedJSON, err := executeCLI("vendors", "list", "--json", "--deleted", dbPath)
	require.NoError(t, err)
	var deletedRows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(deletedJSON), &deletedRows))
	require.Len(t, deletedRows, 1)
	assert.Equal(t, vendorID, deletedRows[0]["id"])
	assert.Equal(t, "Acme Plumbing and Heating", deletedRows[0]["name"])
	assert.NotEmpty(t, deletedRows[0]["deleted_at"])

	_, err = executeCLI("vendors", "restore", vendorID, dbPath)
	require.NoError(t, err)

	activeJSON, err := executeCLI("vendors", "list", "--json", dbPath)
	require.NoError(t, err)
	var activeRows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(activeJSON), &activeRows))
	require.Len(t, activeRows, 1)
	assert.Equal(t, vendorID, activeRows[0]["id"])
	assert.Equal(t, "Acme Plumbing and Heating", activeRows[0]["name"])
}

func TestEditHouseCreateAndUpdateCLI(t *testing.T) {
	t.Parallel()
	dbPath := createTestDB(t)

	_, err := executeCLI(
		"house",
		"add",
		"--data",
		`{"nickname":"The Bungalow","city":"Madison"}`,
		dbPath,
	)
	require.NoError(t, err)

	_, err = executeCLI(
		"house",
		"edit",
		"--data",
		`{"nickname":"The Painted Bungalow","city":"Madison","state":"WI"}`,
		dbPath,
	)
	require.NoError(t, err)

	houseJSON, err := executeCLI("house", "list", "--json", dbPath)
	require.NoError(t, err)
	var house map[string]any
	require.NoError(t, json.Unmarshal([]byte(houseJSON), &house))
	assert.Equal(t, "The Painted Bungalow", house["nickname"])
	assert.Equal(t, "WI", house["state"])
}

func TestEditCreateRequiresPayload(t *testing.T) {
	t.Parallel()
	dbPath := createTestDB(t)

	_, err := executeCLI("vendors", "add", dbPath)
	require.Error(t, err)
	assert.ErrorContains(t, err, "--data or --data-file")
}

func TestEditEntityCRUDCoverage(t *testing.T) {
	t.Parallel()
	dbPath := createTestDB(t)

	projectTypeID := firstID(t, dbPath, "project-types")
	maintenanceCategoryID := firstID(t, dbPath, "maintenance-categories")

	// projects
	projectOut, err := executeCLI(
		"projects",
		"add",
		"--data",
		fmt.Sprintf(`{"title":"Deck Build","project_type_id":"%s","status":"planned"}`, projectTypeID),
		dbPath,
	)
	require.NoError(t, err)
	projectID := strings.TrimSpace(projectOut)
	require.NotEmpty(t, projectID)

	_, err = executeCLI(
		"projects",
		"edit",
		projectID,
		"--data",
		fmt.Sprintf(`{"title":"Deck Build - Phase 1","project_type_id":"%s","status":"in_progress"}`, projectTypeID),
		dbPath,
	)
	require.NoError(t, err)

	// appliances
	applianceOut, err := executeCLI(
		"appliances",
		"add",
		"--data",
		`{"name":"Water Heater","brand":"Acme","location":"Basement"}`,
		dbPath,
	)
	require.NoError(t, err)
	applianceID := strings.TrimSpace(applianceOut)
	require.NotEmpty(t, applianceID)

	_, err = executeCLI(
		"appliances",
		"edit",
		applianceID,
		"--data",
		`{"name":"Water Heater","brand":"Acme","location":"Utility Room"}`,
		dbPath,
	)
	require.NoError(t, err)

	// maintenance
	maintenanceOut, err := executeCLI(
		"maintenance",
		"add",
		"--data",
		fmt.Sprintf(`{"name":"Flush Water Heater","category_id":"%s","season":"fall","interval_months":12}`, maintenanceCategoryID),
		dbPath,
	)
	require.NoError(t, err)
	maintenanceID := strings.TrimSpace(maintenanceOut)
	require.NotEmpty(t, maintenanceID)

	_, err = executeCLI(
		"maintenance",
		"edit",
		maintenanceID,
		"--data",
		fmt.Sprintf(`{"name":"Flush Water Heater - Annual","category_id":"%s","season":"fall","interval_months":12}`, maintenanceCategoryID),
		dbPath,
	)
	require.NoError(t, err)

	// vendors
	vendorOut, err := executeCLI(
		"vendors",
		"add",
		"--data",
		`{"name":"Builder Bros","email":"ops@builder.example"}`,
		dbPath,
	)
	require.NoError(t, err)
	vendorID := strings.TrimSpace(vendorOut)
	require.NotEmpty(t, vendorID)

	// quotes
	quoteOut, err := executeCLI(
		"quotes",
		"add",
		"--data",
		fmt.Sprintf(`{"project_id":"%s","vendor_id":"%s","total_cents":250000}`, projectID, vendorID),
		dbPath,
	)
	require.NoError(t, err)
	quoteID := strings.TrimSpace(quoteOut)
	require.NotEmpty(t, quoteID)

	_, err = executeCLI(
		"quotes",
		"edit",
		quoteID,
		"--data",
		fmt.Sprintf(`{"project_id":"%s","vendor_id":"%s","total_cents":275000,"notes":"updated estimate"}`, projectID, vendorID),
		dbPath,
	)
	require.NoError(t, err)

	// incidents
	incidentOut, err := executeCLI(
		"incidents",
		"add",
		"--data",
		fmt.Sprintf(`{"title":"Leak in utility room","status":"open","severity":"soon","date_noticed":"%s","appliance_id":"%s","vendor_id":"%s"}`,
			time.Now().Format(time.RFC3339), applianceID, vendorID),
		dbPath,
	)
	require.NoError(t, err)
	incidentID := strings.TrimSpace(incidentOut)
	require.NotEmpty(t, incidentID)

	_, err = executeCLI(
		"incidents",
		"edit",
		incidentID,
		"--data",
		fmt.Sprintf(`{"title":"Leak in utility room","status":"in_progress","severity":"soon","date_noticed":"%s","appliance_id":"%s","vendor_id":"%s"}`,
			time.Now().Format(time.RFC3339), applianceID, vendorID),
		dbPath,
	)
	require.NoError(t, err)

	// service-log
	serviceLogOut, err := executeCLI(
		"service-log",
		"add",
		"--data",
		fmt.Sprintf(`{"maintenance_item_id":"%s","serviced_at":"%s","vendor_id":"%s","cost_cents":12000}`,
			maintenanceID, time.Now().Format(time.RFC3339), vendorID),
		dbPath,
	)
	require.NoError(t, err)
	serviceLogID := strings.TrimSpace(serviceLogOut)
	require.NotEmpty(t, serviceLogID)

	_, err = executeCLI(
		"service-log",
		"edit",
		serviceLogID,
		"--data",
		fmt.Sprintf(`{"maintenance_item_id":"%s","serviced_at":"%s","vendor_id":"%s","cost_cents":12500}`,
			maintenanceID, time.Now().Format(time.RFC3339), vendorID),
		dbPath,
	)
	require.NoError(t, err)

	// documents
	documentOut, err := executeCLI(
		"documents",
		"add",
		"--data",
		fmt.Sprintf(`{"title":"Deck Contract","file_name":"deck.pdf","entity_kind":"project","entity_id":"%s","mime_type":"application/pdf","size_bytes":0}`, projectID),
		dbPath,
	)
	require.NoError(t, err)
	documentID := strings.TrimSpace(documentOut)
	require.NotEmpty(t, documentID)

	_, err = executeCLI(
		"documents",
		"edit",
		documentID,
		"--data",
		fmt.Sprintf(`{"title":"Deck Contract v2","file_name":"deck-v2.pdf","entity_kind":"project","entity_id":"%s"}`, projectID),
		dbPath,
	)
	require.NoError(t, err)

	ordered := []struct {
		entity string
		id     string
	}{
		{entity: "quotes", id: quoteID},
		{entity: "documents", id: documentID},
		{entity: "service-log", id: serviceLogID},
		{entity: "maintenance", id: maintenanceID},
		{entity: "incidents", id: incidentID},
		{entity: "appliances", id: applianceID},
		{entity: "projects", id: projectID},
		{entity: "vendors", id: vendorID},
	}

	for _, tc := range ordered {
		_, err = executeCLI(tc.entity, "delete", tc.id, dbPath)
		require.NoError(t, err, "delete %s", tc.entity)
	}

	for i := len(ordered) - 1; i >= 0; i-- {
		tc := ordered[i]
		_, err = executeCLI(tc.entity, "restore", tc.id, dbPath)
		require.NoError(t, err, "restore %s", tc.entity)
	}
}
