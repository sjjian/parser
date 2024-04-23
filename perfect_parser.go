package parser

import (
	"bytes"
	"fmt"
	"github.com/pingcap/parser/ast"
)

// PerfectParse parses a query string to raw ast.StmtNode. support parses query string
// who contains unparsed SQL, the unparsed SQL will be parses to ast.UnparsedStmt.
func (parser *Parser) PerfectParse(sql, charset, collation string) (stmt []ast.StmtNode, warns []error, err error) {
	_, warns, err = parser.Parse(sql, charset, collation)
	stmts := parser.result
	parser.updateStartLineWithOffset(stmts)
	//fmt.Printf("stmts sql[:parser.lexer.stmtStartPos]: %v\n", sql[:parser.lexer.stmtStartPos])
	fmt.Printf("parser.lexer.stmtStartPos: %v\n", parser.lexer.stmtStartPos)
	fmt.Printf("getLineNumber(sql, parser.lexer.stmtStartPos): %v\n", getLineNumber(sql, parser.lexer.stmtStartPos))
	if err == nil {
		return stmts, warns, nil
	}
	// if err is not nil, the query string must be contains unparsed sql.

	if len(stmts) > 0 {
		for _, stmt := range stmts {
			ast.SetFlag(stmt)
		}
		stmt = append(stmt, stmts...)
	}
	// The origin SQL text(input args `sql`) consists of many SQL segments,
	// each SQL segments is a complete SQL and be parsed into `ast.StmtNode`.
	//
	//     good SQL segment       bad SQL segment
	// |---------------------|---------------------|---------------------|---------------------|    origin SQL text
	//			     		 ^				^
	//		            stmtStartPos   lastScanOffset
	//										|------|---------------------|---------------------|    remaining SQL text
	//
	//                       |<   unparsed stmt   >|<          continue to parse it           >|

	start := parser.lexer.stmtStartPos
	cur := parser.lexer.lastScanOffset

	remainingSql := sql[cur:]
	l := NewScanner(remainingSql)
	var v yySymType
	var endOffset int
	var scanEnd = 0
	var defaultDelimiter int = ';'
	delimiter := defaultDelimiter
ScanLoop:
	for {
		result := l.Lex(&v)
		switch result {
		case scanEnd:
			endOffset = l.lastScanOffset - 1
			break ScanLoop
		case delimiter:
			endOffset = l.lastScanOffset
			break ScanLoop
		case begin:
			// ref: https://dev.mysql.com/doc/refman/8.0/en/begin-end.html
			// ref: https://dev.mysql.com/doc/refman/8.0/en/stored-programs-defining.html
			// Support match:
			// BEGIN
			// ...
			// END;
			//
			delimiter = scanEnd
		case end:
			// match `end;`
			var ny yySymType
			next := l.Lex(&ny)
			if next == defaultDelimiter {
				delimiter = defaultDelimiter
				endOffset = l.lastScanOffset
				break ScanLoop
			}
		case invalid:
			// `Lex`内`scan`在进行token遍历时，当有特殊字符时返回invalid，此时未调用`inc`进行滑动，导致每次遍历同一个pos点位触发死循环。有多种情况会返回invalid。
			// 对于解析器本身没影响，因为 token 提取失败就退出了，但是我们需要继续遍历。
			if l.lastScanOffset == l.r.p.Offset {
				l.r.inc()
			}
		}
	}
	unparsedStmtBuf := bytes.Buffer{}
	unparsedStmtBuf.WriteString(sql[start:cur])
	unparsedStmtBuf.WriteString(remainingSql[:endOffset+1])

	fmt.Printf("unparsedStmtBuf.String(): %v\n", unparsedStmtBuf.String())
	fmt.Printf("countNewLinePrefix(unparsedStmtBuf.String()): %v\n", countNewLinePrefix(unparsedStmtBuf.String()))

	//if parser.endLineOffset == 0 {
	//	parser.endLineOffset = 1
	//}

	if start != 0 {
		parser.endLineOffset += getLineNumber(sql, start)
	}

	unparsedSql := unparsedStmtBuf.String()
	parser.endLineOffset += getTotalLine(unparsedStmtBuf.String())

	parser.startLineOffset = parser.endLineOffset + countNewLinePrefix(unparsedStmtBuf.String()) - 1
	if parser.startLineOffset == -1 {
		parser.startLineOffset = 0
	}

	if len(unparsedSql) > 0 {
		un := &ast.UnparsedStmt{}
		un.SetStartLine(parser.startLineOffset + 1)
		un.SetText(unparsedSql)
		fmt.Printf("un.StartLine(): %v\n", un.StartLine())
		stmt = append(stmt, un)
	}

	if len(remainingSql) > endOffset {
		cStmt, cWarn, cErr := parser.PerfectParse(remainingSql[endOffset+1:], charset, collation)
		warns = append(warns, cWarn...)
		if len(cStmt) > 0 {
			stmt = append(stmt, cStmt...)
		}
		if cErr == nil {
			return stmt, warns, cErr
		}
	}
	return stmt, warns, nil
}

func getTotalLine(remainingSql string) int {
	count := 0
	for _, char := range remainingSql {
		if char == '\n' {
			count++
		}
	}
	return count
}

func (parser *Parser) updateStartLineWithOffset(stmts []ast.StmtNode) {
	for i := range stmts {
		fmt.Printf("stmts[i].StartLine(): %v\n", stmts[i].StartLine())
		stmts[i].SetStartLine(stmts[i].StartLine() + parser.startLineOffset)
	}
}

func getLineNumber(s string, pos int) int {
	if pos == 0 {
		return 0
	}

	lineNumber := 0
	for i := 0; i <= pos; i++ {
		if s[i] == '\n' {
			lineNumber++
		}
	}
	return lineNumber
}

func countNewLinePrefix(s string) int {
	count := 0
	for _, char := range s {
		if char == '\n' {
			count++
		} else {
			break
		}
	}
	return count
}
