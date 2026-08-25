package parser_test

import (
	"testing"

	"github.com/pingcap/parser"
	"github.com/pingcap/parser/ast"
)

func TestMySQLCTEParseAndAST(t *testing.T) {
	p := parser.New()

	cases := []struct {
		name          string
		sql           string
		wantRecursive bool
		wantCTENames  []string
		check         func(t *testing.T, stmt ast.StmtNode)
	}{
		{
			name:          "non-recursive dql",
			sql:           "WITH cte AS (SELECT 1) SELECT * FROM cte",
			wantRecursive: false,
			wantCTENames:  []string{"cte"},
			check: func(t *testing.T, stmt ast.StmtNode) {
				sel, ok := stmt.(*ast.SelectStmt)
				if !ok {
					t.Fatalf("expect *ast.SelectStmt, got %T", stmt)
				}
				if sel.With == nil {
					t.Fatal("SelectStmt.With is nil")
				}
			},
		},
		{
			name:          "with column list",
			sql:           "WITH cte (a) AS (SELECT 1) SELECT a FROM cte",
			wantRecursive: false,
			wantCTENames:  []string{"cte"},
			check: func(t *testing.T, stmt ast.StmtNode) {
				sel := stmt.(*ast.SelectStmt)
				if len(sel.With.CTEs[0].ColNameList) != 1 || sel.With.CTEs[0].ColNameList[0].O != "a" {
					t.Fatalf("unexpected ColNameList: %+v", sel.With.CTEs[0].ColNameList)
				}
			},
		},
		{
			name:          "recursive",
			sql:           "WITH RECURSIVE cte AS (SELECT 1 AS n UNION SELECT n+1 FROM cte WHERE n < 3) SELECT * FROM cte",
			wantRecursive: true,
			wantCTENames:  []string{"cte"},
			check: func(t *testing.T, stmt ast.StmtNode) {
				sel, ok := stmt.(*ast.SelectStmt)
				if !ok {
					t.Fatalf("expect *ast.SelectStmt, got %T", stmt)
				}
				if sel.With == nil {
					t.Fatal("SelectStmt.With is nil")
				}
			},
		},
		{
			name:          "cte + delete",
			sql:           "WITH cte AS (SELECT 1 AS id) DELETE FROM t WHERE id IN (SELECT id FROM cte)",
			wantRecursive: false,
			wantCTENames:  []string{"cte"},
			check: func(t *testing.T, stmt ast.StmtNode) {
				del, ok := stmt.(*ast.DeleteStmt)
				if !ok {
					t.Fatalf("expect *ast.DeleteStmt, got %T", stmt)
				}
				if del.With == nil {
					t.Fatal("DeleteStmt.With is nil")
				}
			},
		},
		{
			name:          "cte + update",
			sql:           "WITH cte AS (SELECT 1 AS id) UPDATE t SET c=1 WHERE id IN (SELECT id FROM cte)",
			wantRecursive: false,
			wantCTENames:  []string{"cte"},
			check: func(t *testing.T, stmt ast.StmtNode) {
				upd, ok := stmt.(*ast.UpdateStmt)
				if !ok {
					t.Fatalf("expect *ast.UpdateStmt, got %T", stmt)
				}
				if upd.With == nil {
					t.Fatal("UpdateStmt.With is nil")
				}
			},
		},
		{
			name:          "cte + union",
			sql:           "WITH cte AS (SELECT 1 AS n) SELECT n FROM cte UNION SELECT 2",
			wantRecursive: false,
			wantCTENames:  []string{"cte"},
			check: func(t *testing.T, stmt ast.StmtNode) {
				u, ok := stmt.(*ast.UnionStmt)
				if !ok {
					t.Fatalf("expect *ast.UnionStmt, got %T", stmt)
				}
				if u.With == nil {
					t.Fatal("UnionStmt.With is nil")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmt, err := p.ParseOneStmt(tc.sql, "", "")
			if err != nil {
				t.Fatalf("ParseOneStmt failed: %v", err)
			}
			if _, ok := stmt.(*ast.UnparsedStmt); ok {
				t.Fatal("got UnparsedStmt; CTE must be positively parsed")
			}
			tc.check(t, stmt)

			var with *ast.WithClause
			switch s := stmt.(type) {
			case *ast.SelectStmt:
				with = s.With
			case *ast.UnionStmt:
				with = s.With
			case *ast.DeleteStmt:
				with = s.With
			case *ast.UpdateStmt:
				with = s.With
			}
			if with == nil {
				t.Fatal("WithClause missing on statement")
			}
			if with.IsRecursive != tc.wantRecursive {
				t.Fatalf("IsRecursive=%v, want %v", with.IsRecursive, tc.wantRecursive)
			}
			if len(with.CTEs) != len(tc.wantCTENames) {
				t.Fatalf("CTEs len=%d, want %d", len(with.CTEs), len(tc.wantCTENames))
			}
			for i, name := range tc.wantCTENames {
				cte := with.CTEs[i]
				if cte.Name.L != name {
					t.Fatalf("CTE[%d] name=%q, want %q", i, cte.Name.L, name)
				}
				if cte.Query == nil {
					t.Fatalf("CTE[%d] Query is nil", i)
				}
			}
		})
	}
}

func TestMySQLCTENonRegressionWithGrantOption(t *testing.T) {
	p := parser.New()
	// WITH GRANT OPTION must not be parsed as CTE.
	sql := "GRANT SELECT ON db.t TO 'u'@'%' WITH GRANT OPTION"
	stmt, err := p.ParseOneStmt(sql, "", "")
	if err != nil {
		t.Fatalf("ParseOneStmt failed: %v", err)
	}
	if _, ok := stmt.(*ast.GrantStmt); !ok {
		t.Fatalf("expect *ast.GrantStmt, got %T", stmt)
	}
}
