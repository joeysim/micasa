// Copyright 2026 Phillip Cloud
// Licensed under the Apache License, Version 2.0

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/micasa-dev/micasa/internal/data"
	"github.com/spf13/cobra"
)

type editOptions struct {
	op       string
	id       string
	jsonData string
	dataFile string
}

type showListFn func(w io.Writer, store *data.Store, asJSON, includeDeleted bool) error

type mutateFn func(cmd *cobra.Command, store *data.Store, opts editOptions) error

func newEntityCommands() []*cobra.Command {
	return []*cobra.Command{
		newEntityCommand("house", showHouseList, runHouseEdit, false),
		newEntityCommand("projects", showProjects, runProjectEdit, true),
		newEntityCommand("quotes", showQuotes, runQuoteEdit, true),
		newEntityCommand("maintenance", showMaintenance, runMaintenanceEdit, true),
		newEntityCommand("service-log", showServiceLog, runServiceLogEdit, true),
		newEntityCommand("appliances", showAppliances, runApplianceEdit, true),
		newEntityCommand("incidents", showIncidents, runIncidentEdit, true),
		newEntityCommand("vendors", showVendors, runVendorEdit, true),
		newEntityCommand("documents", showDocuments, runDocumentEdit, true),
	}
}

func showHouseList(w io.Writer, store *data.Store, asJSON, _ bool) error {
	return showHouse(w, store, asJSON)
}

func newEntityCommand(
	name string,
	listFn showListFn,
	runner mutateFn,
	supportsRestore bool,
) *cobra.Command {
	short := "Manage " + name + " (list/add/edit/delete/restore; add/edit use --data or --data-file)"
	if name == "house" {
		short = "Manage house (list/add/edit; add/edit use --data or --data-file)"
	}
	entityCmd := &cobra.Command{
		Use:           name,
		Short:         short,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	editNeedsID := name != "house"
	entityCmd.AddCommand(newListCmd(listFn), newAddCmd(runner), newEditCmd(runner, editNeedsID))
	if name != "house" {
		entityCmd.AddCommand(newDeleteCmd(runner))
	}
	if supportsRestore && name != "house" {
		entityCmd.AddCommand(newRestoreCmd(runner))
	}
	return entityCmd
}

func newListCmd(listFn showListFn) *cobra.Command {
	var jsonFlag bool
	var deletedFlag bool

	cmd := &cobra.Command{
		Use:           "list [database-path]",
		Short:         "List entities",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openExisting(dbPathFromEnvOrArg(args))
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()
			return listFn(cmd.OutOrStdout(), store, jsonFlag, deletedFlag)
		},
	}

	cmd.Flags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&deletedFlag, "deleted", false, "Include soft-deleted rows")
	return cmd
}

func newAddCmd(runner mutateFn) *cobra.Command {
	opts := editOptions{op: "create"}
	cmd := &cobra.Command{
		Use:           "add [database-path]",
		Short:         "Create an entity",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openExisting(dbPathFromEnvOrArg(args))
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()
			return runner(cmd, store, opts)
		},
	}
	cmd.Flags().StringVar(&opts.jsonData, "data", "", "Inline JSON payload")
	cmd.Flags().StringVar(&opts.dataFile, "data-file", "", "Path to JSON payload file")
	return cmd
}

func newEditCmd(runner mutateFn, requiresID bool) *cobra.Command {
	opts := editOptions{op: "update"}
	use := "edit <id> [database-path]"
	args := cobra.RangeArgs(1, 2)
	if !requiresID {
		use = "edit [database-path]"
		args = cobra.MaximumNArgs(1)
	}
	cmd := &cobra.Command{
		Use:           use,
		Short:         "Update an entity",
		Args:          args,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbArgs := args
			opts.id = ""
			if requiresID {
				opts.id = strings.TrimSpace(args[0])
				dbArgs = args[1:]
			}
			store, err := openExisting(dbPathFromEnvOrArg(dbArgs))
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()
			return runner(cmd, store, opts)
		},
	}
	cmd.Flags().StringVar(&opts.jsonData, "data", "", "Inline JSON payload")
	cmd.Flags().StringVar(&opts.dataFile, "data-file", "", "Path to JSON payload file")
	return cmd
}

func newDeleteCmd(runner mutateFn) *cobra.Command {
	opts := editOptions{op: "delete"}
	return &cobra.Command{
		Use:           "delete <id> [database-path]",
		Short:         "Soft-delete an entity",
		Args:          cobra.RangeArgs(1, 2),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.id = strings.TrimSpace(args[0])
			store, err := openExisting(dbPathFromEnvOrArg(args[1:]))
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()
			return runner(cmd, store, opts)
		},
	}
}

func newRestoreCmd(runner mutateFn) *cobra.Command {
	opts := editOptions{op: "restore"}
	return &cobra.Command{
		Use:           "restore <id> [database-path]",
		Short:         "Restore a soft-deleted entity",
		Args:          cobra.RangeArgs(1, 2),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.id = strings.TrimSpace(args[0])
			store, err := openExisting(dbPathFromEnvOrArg(args[1:]))
			if err != nil {
				return err
			}
			defer func() { _ = store.Close() }()
			return runner(cmd, store, opts)
		},
	}
}

func payload(opts editOptions) ([]byte, error) {
	if strings.TrimSpace(opts.jsonData) != "" {
		if strings.TrimSpace(opts.dataFile) != "" {
			return nil, errors.New("use only one of --data or --data-file")
		}
		return []byte(opts.jsonData), nil
	}
	if strings.TrimSpace(opts.dataFile) != "" {
		b, err := os.ReadFile(opts.dataFile)
		if err != nil {
			return nil, fmt.Errorf("read --data-file: %w", err)
		}
		return b, nil
	}
	return nil, errors.New("--data or --data-file is required for add/edit")
}

func mustID(opts editOptions) error {
	if strings.TrimSpace(opts.id) == "" {
		return errors.New("id is required")
	}
	return nil
}

func decodeInto[T any](opts editOptions, out *T) error {
	b, err := payload(opts)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("decode JSON payload: %w", err)
	}
	return nil
}

func runHouseEdit(_ *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var h data.HouseProfile
		if err := decodeInto(opts, &h); err != nil {
			return err
		}
		return store.CreateHouseProfile(h)
	case "update":
		var h data.HouseProfile
		if err := decodeInto(opts, &h); err != nil {
			return err
		}
		return store.UpdateHouseProfile(h)
	default:
		return errors.New("house supports only add, edit, and list")
	}
}

func runVendorEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var v data.Vendor
		if err := decodeInto(opts, &v); err != nil {
			return err
		}
		if err := store.CreateVendor(&v); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), v.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var v data.Vendor
		if err := decodeInto(opts, &v); err != nil {
			return err
		}
		v.ID = opts.id
		if err := store.UpdateVendor(v); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), v.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteVendor(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreVendor(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func runProjectEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	return runSimpleCRUDProject(cmd, store, opts)
}
func runApplianceEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	return runSimpleCRUDAppliance(cmd, store, opts)
}
func runMaintenanceEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	return runSimpleCRUDMaintenance(cmd, store, opts)
}
func runIncidentEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	return runSimpleCRUDIncident(cmd, store, opts)
}
func runDocumentEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	return runSimpleCRUDDocument(cmd, store, opts)
}

func runSimpleCRUDProject(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var p data.Project
		if err := decodeInto(opts, &p); err != nil {
			return err
		}
		if err := store.CreateProject(&p); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), p.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var p data.Project
		if err := decodeInto(opts, &p); err != nil {
			return err
		}
		p.ID = opts.id
		if err := store.UpdateProject(p); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), p.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteProject(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreProject(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func runSimpleCRUDAppliance(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var a data.Appliance
		if err := decodeInto(opts, &a); err != nil {
			return err
		}
		if err := store.CreateAppliance(&a); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), a.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var a data.Appliance
		if err := decodeInto(opts, &a); err != nil {
			return err
		}
		a.ID = opts.id
		if err := store.UpdateAppliance(a); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), a.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteAppliance(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreAppliance(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func runSimpleCRUDMaintenance(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var m data.MaintenanceItem
		if err := decodeInto(opts, &m); err != nil {
			return err
		}
		if err := store.CreateMaintenance(&m); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), m.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var m data.MaintenanceItem
		if err := decodeInto(opts, &m); err != nil {
			return err
		}
		m.ID = opts.id
		if err := store.UpdateMaintenance(m); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), m.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteMaintenance(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreMaintenance(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func runSimpleCRUDIncident(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var i data.Incident
		if err := decodeInto(opts, &i); err != nil {
			return err
		}
		if err := store.CreateIncident(&i); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), i.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var i data.Incident
		if err := decodeInto(opts, &i); err != nil {
			return err
		}
		i.ID = opts.id
		if err := store.UpdateIncident(i); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), i.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteIncident(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreIncident(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func runSimpleCRUDDocument(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var d data.Document
		if err := decodeInto(opts, &d); err != nil {
			return err
		}
		if err := store.CreateDocument(&d); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), d.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var d data.Document
		if err := decodeInto(opts, &d); err != nil {
			return err
		}
		d.ID = opts.id
		if err := store.UpdateDocument(d); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), d.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteDocument(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreDocument(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func runQuoteEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var q data.Quote
		if err := decodeInto(opts, &q); err != nil {
			return err
		}
		vendor, err := store.GetVendor(q.VendorID)
		if err != nil {
			return fmt.Errorf("resolve vendor: %w", err)
		}
		if err := store.CreateQuote(&q, vendor); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), q.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var q data.Quote
		if err := decodeInto(opts, &q); err != nil {
			return err
		}
		q.ID = opts.id
		vendor, err := store.GetVendor(q.VendorID)
		if err != nil {
			return fmt.Errorf("resolve vendor: %w", err)
		}
		if err := store.UpdateQuote(q, vendor); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), q.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteQuote(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreQuote(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func runServiceLogEdit(cmd *cobra.Command, store *data.Store, opts editOptions) error {
	switch opts.op {
	case "create":
		var s data.ServiceLogEntry
		if err := decodeInto(opts, &s); err != nil {
			return err
		}
		vendor, err := serviceLogVendor(store, s.VendorID)
		if err != nil {
			return err
		}
		if err := store.CreateServiceLog(&s, vendor); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), s.ID)
		return nil
	case "update":
		if err := mustID(opts); err != nil {
			return err
		}
		var s data.ServiceLogEntry
		if err := decodeInto(opts, &s); err != nil {
			return err
		}
		s.ID = opts.id
		vendor, err := serviceLogVendor(store, s.VendorID)
		if err != nil {
			return err
		}
		if err := store.UpdateServiceLog(s, vendor); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), s.ID)
		return nil
	case "delete":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.DeleteServiceLog(opts.id)
	case "restore":
		if err := mustID(opts); err != nil {
			return err
		}
		return store.RestoreServiceLog(opts.id)
	default:
		return errors.New("unsupported operation")
	}
}

func serviceLogVendor(store *data.Store, vendorID *string) (data.Vendor, error) {
	if vendorID == nil || strings.TrimSpace(*vendorID) == "" {
		return data.Vendor{}, nil
	}
	vendor, err := store.GetVendor(*vendorID)
	if err != nil {
		return data.Vendor{}, fmt.Errorf("resolve vendor: %w", err)
	}
	return vendor, nil
}
