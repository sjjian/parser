package parser_test

import (
	"strings"
	"testing"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/format"
	_ "github.com/pingcap/parser/test_driver"
)

type supersetNopVisitor struct{}

func (supersetNopVisitor) Enter(in ast.Node) (ast.Node, bool) { return in, false }
func (supersetNopVisitor) Leave(in ast.Node) (ast.Node, bool) { return in, true }

// S2 AC-010 / AC-011: Parse / Restore / Restore∘Parse / Accept for first-slice features.
func TestSupersetParseRestoreAccept(t *testing.T) {
	cases := []struct {
		feature string
		sql     string
		wantTyp string
	}{
		{"import_into", `IMPORT INTO t FROM '/data/t.csv'`, "*ast.ImportIntoStmt"},
		{"import_into", `IMPORT INTO t (id, name) FROM '/data/t.csv'`, "*ast.ImportIntoStmt"},
		{"expr_default", `CREATE TABLE t1 (id int NOT NULL, j json NOT NULL DEFAULT (JSON_ARRAY()))`, "*ast.CreateTableStmt"},
		{"expr_default", `CREATE TABLE t4 (id int NOT NULL, n int NOT NULL DEFAULT ((1 + 2)))`, "*ast.CreateTableStmt"},
		{"fk_ga", `CREATE TABLE c (id INT, pid INT, INDEX idx_pid (pid), FOREIGN KEY (pid) REFERENCES p(id) ON DELETE CASCADE)`, "*ast.CreateTableStmt"},
		{"fk_ga", `ALTER TABLE c ADD CONSTRAINT fk FOREIGN KEY (pid) REFERENCES p(id) ON UPDATE SET NULL`, "*ast.AlterTableStmt"},
		{"vector", `CREATE TABLE t (id INT PRIMARY KEY, embedding VECTOR(3))`, "*ast.CreateTableStmt"},
		{"vector", `CREATE TABLE t (id INT PRIMARY KEY, embedding VECTOR)`, "*ast.CreateTableStmt"},
		{"affinity", `CREATE TABLE t1 (a INT) AFFINITY = 'table'`, "*ast.CreateTableStmt"},
		{"show_affinity", `SHOW AFFINITY`, "*ast.ShowStmt"},
		{"for_update_of", `SELECT * FROM t AS a FOR UPDATE OF a`, "*ast.SelectStmt"},
		{"partial_index", `CREATE INDEX idx ON t (c) WHERE c IS NOT NULL`, "*ast.CreateIndexStmt"},
		{"lateral", `SELECT * FROM t1, LATERAL (SELECT * FROM t2 WHERE t2.id = t1.id) AS dt`, "*ast.SelectStmt"},
	}
	p := parser.New()
	for _, c := range cases {
		c := c
		t.Run(c.feature+"/"+c.sql, func(t *testing.T) {
			stmts, _, err := p.Parse(c.sql, "", "")
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(stmts) == 0 {
				t.Fatal("Parse: empty stmts")
			}
			if _, ok := stmts[0].(*ast.UnparsedStmt); ok {
				t.Fatalf("Parse landed on UnparsedStmt (not real AST)")
			}
			gotTyp := ""
			switch stmts[0].(type) {
			case *ast.ImportIntoStmt:
				gotTyp = "*ast.ImportIntoStmt"
			case *ast.CreateTableStmt:
				gotTyp = "*ast.CreateTableStmt"
			case *ast.AlterTableStmt:
				gotTyp = "*ast.AlterTableStmt"
			case *ast.ShowStmt:
				gotTyp = "*ast.ShowStmt"
			case *ast.SelectStmt:
				gotTyp = "*ast.SelectStmt"
			case *ast.CreateIndexStmt:
				gotTyp = "*ast.CreateIndexStmt"
			default:
				gotTyp = "other"
			}
			if gotTyp != c.wantTyp {
				t.Fatalf("type=%T want %s", stmts[0], c.wantTyp)
			}
			var b strings.Builder
			ctx := format.NewRestoreCtx(format.DefaultRestoreFlags, &b)
			if err := stmts[0].Restore(ctx); err != nil {
				t.Fatalf("Restore: %v", err)
			}
			restored := b.String()
			if restored == "" {
				t.Fatal("Restore empty")
			}
			stmts2, _, err := p.Parse(restored, "", "")
			if err != nil {
				t.Fatalf("Restore∘Parse: %v restored=%s", err, restored)
			}
			if _, ok := stmts2[0].(*ast.UnparsedStmt); ok {
				t.Fatalf("Restore∘Parse UnparsedStmt restored=%s", restored)
			}
			if _, ok := stmts[0].Accept(supersetNopVisitor{}); !ok {
				t.Fatal("Accept returned !ok")
			}
		})
	}
}
