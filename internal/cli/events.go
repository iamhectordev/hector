package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/doron-cohen/klee"
	dbsqlite "github.com/iamhectordev/hector/internal/db/sqlite"
	"github.com/iamhectordev/hector/pkg/waffle"
	wafflesqlite "github.com/iamhectordev/hector/pkg/waffle/sqlite"
	"github.com/urfave/cli/v3"
)

func eventsCommand() *cli.Command {
	return &cli.Command{
		Name:  "events",
		Usage: "inspect stored events",
		Commands: []*cli.Command{
			eventsListCommand(),
			eventsGetCommand(),
		},
	}
}

func eventsListCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "list recent events",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "limit",
				Usage: "max events to return",
				Value: 100,
			},
			&cli.StringFlag{
				Name:  "before",
				Usage: "return events before RFC3339 time (exclusive)",
			},
		},
		Action: eventsListAction,
	}
}

func eventsGetCommand() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "get one event by id",
		ArgsUsage: "<event-id>",
		Action:    eventsGetAction,
	}
}

func eventsListAction(ctx context.Context, cmd *cli.Command) error {
	reader, closeDB, err := openEventReader(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	query := waffle.EventQuery{Limit: cmd.Int("limit")}
	if rawBefore := strings.TrimSpace(cmd.String("before")); rawBefore != "" {
		before, parseErr := time.Parse(time.RFC3339, rawBefore)
		if parseErr != nil {
			return fmt.Errorf("invalid --before value %q (expected RFC3339, e.g. 2026-05-10T15:00:00Z)", rawBefore)
		}
		query.Before = before
	}

	events, err := reader.List(ctx, query)
	if err != nil {
		return err
	}

	if klee.GetRunFlags(ctx).JSON {
		return writeJSON(cmd.Root().Writer, toEventOutputs(events))
	}

	return writeEventsListHuman(cmd.Root().Writer, events)
}

func eventsGetAction(ctx context.Context, cmd *cli.Command) error {
	id := strings.TrimSpace(cmd.Args().First())
	if id == "" {
		return fmt.Errorf("missing event id (usage: hector events get <event-id>)")
	}

	reader, closeDB, err := openEventReader(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	event, err := reader.Get(ctx, id)
	if err != nil {
		if errors.Is(err, waffle.ErrEventNotFound) {
			return fmt.Errorf("event not found: %s", id)
		}
		return err
	}

	if klee.GetRunFlags(ctx).JSON {
		return writeJSON(cmd.Root().Writer, toEventOutput(event))
	}

	return writeEventHuman(cmd.Root().Writer, event)
}

func openEventReader(ctx context.Context) (*waffle.Reader, func(), error) {
	cfg, err := configFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	db, err := dbsqlite.Open(ctx, cfg.DB)
	if err != nil {
		return nil, nil, err
	}

	closeDB := func() {
		_ = db.Close()
	}

	if err := dbsqlite.Migrate(ctx, db, wafflesqlite.Migrations()); err != nil {
		closeDB()
		return nil, nil, fmt.Errorf("waffle migrations: %w", err)
	}

	return waffle.NewReader(wafflesqlite.NewStore(db)), closeDB, nil
}

type eventOutput struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	SchemaVersion int         `json:"schema_version"`
	OccurredAt    string      `json:"occurred_at"`
	Payload       interface{} `json:"payload"`
}

func toEventOutputs(events []waffle.EventRecord) []eventOutput {
	out := make([]eventOutput, 0, len(events))
	for _, event := range events {
		out = append(out, toEventOutput(event))
	}
	return out
}

func toEventOutput(event waffle.EventRecord) eventOutput {
	return eventOutput{
		ID:            event.ID,
		Type:          event.Type,
		SchemaVersion: event.SchemaVersion,
		OccurredAt:    event.OccurredAt.UTC().Format(time.RFC3339),
		Payload:       decodePayload(event.Payload),
	}
}

func decodePayload(payload []byte) interface{} {
	var decoded interface{}
	if err := json.Unmarshal(payload, &decoded); err == nil {
		return decoded
	}
	return string(payload)
}

func writeJSON(w io.Writer, value interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writeEventsListHuman(w io.Writer, events []waffle.EventRecord) error {
	if _, err := fmt.Fprintf(w, "Recent events (%d)\n", len(events)); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "ID\tTYPE\tOCCURRED AT (UTC)\tVER"); err != nil {
		return err
	}
	for _, event := range events {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%d\n",
			event.ID,
			event.Type,
			event.OccurredAt.UTC().Format(time.RFC3339),
			event.SchemaVersion,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}

func writeEventHuman(w io.Writer, event waffle.EventRecord) error {
	if _, err := fmt.Fprintln(w, "Event"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  ID:             %s\n", event.ID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Type:           %s\n", event.Type); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Schema version: %d\n", event.SchemaVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Occurred at:    %s\n", event.OccurredAt.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Payload"); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, indentPayload(event.Payload))
	return err
}

func indentPayload(payload []byte) string {
	if len(payload) == 0 {
		return "  null"
	}
	if !json.Valid(payload) {
		return "  " + string(payload)
	}

	var out bytes.Buffer
	if err := json.Indent(&out, payload, "  ", "  "); err != nil {
		return "  " + string(payload)
	}
	return out.String()
}
