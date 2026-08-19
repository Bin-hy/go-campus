# 5 条慢 SQL 优化（explain 自测）

## 难度：⭐⭐ 中等（纯 SQL，无 Go 测试）

## 考点
- EXPLAIN 关键字段（type / key / rows / Extra）
- 索引失效场景（左模糊、隐式类型转换、函数包裹…）
- 覆盖索引、联合索引、深翻页优化

## 题目描述

表结构：

```sql
users(id, name, email, age, created_at)
orders(id, user_id, amount, status, created_at)   -- 已有索引 idx_user(user_id)
```

打开 `exercises.sql`，对 5 条慢 SQL 写出**优化后**写法并说明原因。答案在 `answer/answer.sql`（先自己做再看）。

## 自测方式

有 Docker 环境时（`code/backend` 下有 MySQL）：

```bash
cd code/backend && docker compose up -d mysql
# 然后对每条 SQL 执行 EXPLAIN，对比优化前后 type / rows / Extra
```

没有环境时，对着 `answer/answer.sql` 逐条口述 explain 差异即可。

## 验收
- [ ] 每条 SQL 能说出"优化前 type/rows/Extra 差在哪、优化后好在哪"
- [ ] 能背出索引失效 6 类场景并各举一例
- [ ] 能讲出深翻页的两种优化（游标、延迟关联）及适用边界
- [ ] 能讲出为什么左模糊 `%张%` 用不了索引（B+ 树有序，前缀未知无法定位起始位置）
