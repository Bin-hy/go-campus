package explain

import (
	"context"
	"database/sql"
	"fmt"
)

// explainType 读取 EXPLAIN 结果中的 type 列。
func explainType(ctx context.Context, db *sql.DB, query string) (string, error) {
	rows, err := db.QueryContext(ctx, "EXPLAIN "+query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if !rows.Next() {
		return "", fmt.Errorf("EXPLAIN 无结果")
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	for i, c := range cols {
		if c == "type" {
			if b, ok := vals[i].([]byte); ok {
				return string(b), nil
			}
		}
	}
	return "", fmt.Errorf("未找到 type 列")
}

// CompareExplain 参考答案。
func CompareExplain(ctx context.Context, db *sql.DB) (string, string, error) {
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS t_explain`); err != nil {
		return "", "", err
	}
	if _, err := db.ExecContext(ctx, `CREATE TABLE t_explain (
		id INT PRIMARY KEY AUTO_INCREMENT,
		name VARCHAR(50),
		age INT
	)`); err != nil {
		return "", "", err
	}
	// 插入 500 行 name 各不相同的记录（保证选择性，否则优化器可能仍全表扫）
	for i := 0; i < 500; i++ {
		if _, err := db.ExecContext(ctx, `INSERT INTO t_explain (name, age) VALUES (?, ?)`,
			fmt.Sprintf("user_%d", i), i); err != nil {
			return "", "", err
		}
	}

	query := `SELECT * FROM t_explain WHERE name = 'user_250'`
	before, err := explainType(ctx, db, query)
	if err != nil {
		return "", "", err
	}

	if _, err := db.ExecContext(ctx, `CREATE INDEX idx_name ON t_explain(name)`); err != nil {
		return "", "", err
	}
	after, err := explainType(ctx, db, query)
	if err != nil {
		return "", "", err
	}

	return before, after, nil
}
