package memory_test

// Retrieval quality benchmark against the SciFact corpus.
//
// Usage:
//
//	SCIFACT_CORPUS=/path/to/benchmarks/memory/scifact go test -v -run TestSciFact ./benchmarks/memory/
//
// Prints recall@1/3/10 and nDCG@10 for FTS-only and hybrid (FTS+vec) modes.
// Always passes — the report is the output.

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	_ "modernc.org/sqlite"
	_ "modernc.org/sqlite/vec"

	"github.com/iamhectordev/hector/pkg/memory"
	memorysqlite "github.com/iamhectordev/hector/pkg/memory/sqlite"
	"github.com/iamhectordev/hector/pkg/migrations"
)

func TestSciFact(t *testing.T) {
	dir := os.Getenv("SCIFACT_CORPUS")
	if dir == "" {
		t.Skip("SCIFACT_CORPUS not set — skipping retrieval benchmark")
	}

	ctx := context.Background()

	type record struct {
		ID   string    `json:"id"`
		Text string    `json:"text"`
		Vec  []float32 `json:"vec"`
	}
	type qrel struct {
		QueryID string `json:"query_id"`
		DocID   string `json:"doc_id"`
		Score   int    `json:"score"`
	}

	load := func(name string) []record {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			log.Fatalf("open %s: %v", name, err)
		}
		defer f.Close()
		var out []record
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 4<<20), 4<<20)
		for sc.Scan() {
			var r record
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				log.Fatalf("parse %s: %v", name, err)
			}
			out = append(out, r)
		}
		return out
	}

	corpus := load("corpus.jsonl")
	queries := load("queries.jsonl")
	embeddings := load("embeddings.jsonl")

	var qrels []qrel
	f, _ := os.Open(filepath.Join(dir, "qrels.jsonl"))
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var q qrel
		json.Unmarshal(sc.Bytes(), &q)
		qrels = append(qrels, q)
	}
	f.Close()

	// id → vec
	idToVec := make(map[string][]float32, len(embeddings))
	for _, e := range embeddings {
		idToVec[e.ID] = e.Vec
	}

	// text → vec (for the fixture embedder)
	textToVec := make(map[string][]float32, len(corpus)+len(queries))
	for _, d := range corpus {
		if v, ok := idToVec[d.ID]; ok {
			textToVec[d.Text] = v
		}
	}
	for _, q := range queries {
		if v, ok := idToVec[q.ID]; ok {
			textToVec[q.Text] = v
		}
	}

	// query_id → relevant doc ids
	relevant := make(map[string]map[string]struct{})
	for _, qr := range qrels {
		if qr.Score > 0 {
			if relevant[qr.QueryID] == nil {
				relevant[qr.QueryID] = make(map[string]struct{})
			}
			relevant[qr.QueryID][qr.DocID] = struct{}{}
		}
	}

	// only evaluate queries that have at least one relevant doc
	var evalQueries []record
	for _, q := range queries {
		if len(relevant[q.ID]) > 0 {
			evalQueries = append(evalQueries, q)
		}
	}

	fmt.Printf("\nSciFact  —  %d docs  |  %d queries with qrels\n\n", len(corpus), len(evalQueries))
	fmt.Printf("%-24s  %8s  %8s  %8s  %8s\n", "mode", "recall@1", "recall@3", "recall@10", "nDCG@10")
	fmt.Println("------------------------------------------------------------------------")

	for _, hybrid := range []bool{false, true} {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		runner := migrations.New(db)
		runner.Add(memorysqlite.Migrations())
		if err := runner.Run(ctx); err != nil {
			t.Fatal(err)
		}

		var store *memorysqlite.Store
		if hybrid {
			store = memorysqlite.NewStore(db, memorysqlite.WithEmbedder(&fixedEmbedder{m: textToVec}))
		} else {
			store = memorysqlite.NewStore(db)
		}

		for _, d := range corpus {
			if err := store.Put(ctx, memory.Object{ID: d.ID, Content: d.Text}); err != nil {
				t.Fatalf("put %s: %v", d.ID, err)
			}
		}

		results := make(map[string][]string, len(evalQueries))
		for _, q := range evalQueries {
			objs, err := store.Search(ctx, q.Text, 10)
			if err != nil {
				t.Fatalf("search %s: %v", q.ID, err)
			}
			ids := make([]string, len(objs))
			for i, o := range objs {
				ids[i] = o.ID
			}
			results[q.ID] = ids
		}

		db.Close()

		mode := "FTS only"
		if hybrid {
			mode = "hybrid (FTS + vec0)"
		}
		fmt.Printf("%-24s  %8.3f  %8.3f  %8.3f  %8.3f\n",
			mode,
			recall(results, relevant, 1),
			recall(results, relevant, 3),
			recall(results, relevant, 10),
			ndcg(results, relevant, 10),
		)
	}
	fmt.Println()
}

type fixedEmbedder struct{ m map[string][]float32 }

func (e *fixedEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return e.m[text], nil
}

func recall(results map[string][]string, relevant map[string]map[string]struct{}, k int) float64 {
	var hit float64
	for qid, ids := range results {
		rel := relevant[qid]
		top := ids
		if len(top) > k {
			top = top[:k]
		}
		for _, id := range top {
			if _, ok := rel[id]; ok {
				hit++
				break
			}
		}
	}
	return hit / float64(len(results))
}

func ndcg(results map[string][]string, relevant map[string]map[string]struct{}, k int) float64 {
	var total float64
	for qid, ids := range results {
		rel := relevant[qid]
		top := ids
		if len(top) > k {
			top = top[:k]
		}

		var dcg float64
		for i, id := range top {
			if _, ok := rel[id]; ok {
				dcg += 1.0 / math.Log2(float64(i+2))
			}
		}

		idealLen := len(rel)
		if idealLen > k {
			idealLen = k
		}
		ranks := make([]float64, idealLen)
		for i := range ranks {
			ranks[i] = 1.0 / math.Log2(float64(i+2))
		}
		sort.Sort(sort.Reverse(sort.Float64Slice(ranks)))
		var idcg float64
		for _, v := range ranks {
			idcg += v
		}

		if idcg > 0 {
			total += dcg / idcg
		}
	}
	return total / float64(len(results))
}
