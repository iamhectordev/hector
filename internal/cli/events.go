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

	writeEventsListHuman(cmd.Root().Writer, events)
	return nil
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

	writeEventHuman(cmd.Root().Writer, event)
	return nil
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

func writeEventsListHuman(w io.Writer, events []waffle.EventRecord) {
	fmt.Fprintf(w, "Recent events (%d)\n", len(events))
	if len(events) == 0 {
		return
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tTYPE\tOCCURRED AT (UTC)\tVER")
	for _, event := range events {
		fmt.Fprintf(table, "%s\t%s\t%s\t%d\n",
			event.ID,
			event.Type,
			event.OccurredAt.UTC().Format(time.RFC3339),
			event.SchemaVersion,
		)
	}
	_ = table.Flush()
}

func writeEventHuman(w io.Writer, event waffle.EventRecord) {
	fmt.Fprintln(w, "Event")
	fmt.Fprintf(w, "  ID:             %s\n", event.ID)
	fmt.Fprintf(w, "  Type:           %s\n", event.Type)
	fmt.Fprintf(w, "  Schema version: %d\n", event.SchemaVersion)
	fmt.Fprintf(w, "  Occurred at:    %s\n", event.OccurredAt.UTC().Format(time.RFC3339))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Payload")
	fmt.Fprintln(w, indentPayload(event.Payload))
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
