-- 5 条慢 SQL 优化练习（写出优化后写法，答案见 answer/answer.sql）
--
-- 表结构：
--   users(id, name, email, age, created_at)
--   orders(id, user_id, amount, status, created_at)  -- 已有索引 idx_user(user_id)

-- 慢 SQL 1：排序没用索引（Using filesort）
-- SELECT * FROM orders WHERE status = 1 ORDER BY created_at DESC LIMIT 10;
-- TODO: 写出优化方案（建什么索引？）

-- 慢 SQL 2：左模糊，索引失效（全表扫）
-- SELECT * FROM users WHERE name LIKE '%张%';
-- TODO: 写出优化后写法

-- 慢 SQL 3：隐式类型转换（user_id 是 BIGINT，条件写字符串）
-- SELECT * FROM orders WHERE user_id = '12345';
-- TODO: 写出优化后写法

-- 慢 SQL 4：大范围 + SELECT * 大量回表
-- SELECT * FROM orders WHERE created_at BETWEEN '2024-01-01' AND '2024-06-30';
-- TODO: 写出优化方案（覆盖索引 + 只取必要列）

-- 慢 SQL 5：深翻页（LIMIT 100000, 20 要扫 10 万行）
-- SELECT * FROM orders ORDER BY id LIMIT 100000, 20;
-- TODO: 写出游标分页写法
