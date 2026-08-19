-- 参考答案：5 条慢 SQL 优化
--
-- 表结构：
--   users(id, name, email, age, created_at)
--   orders(id, user_id, amount, status, created_at)  -- 已有索引 idx_user(user_id)

-- 慢 SQL 1：建联合索引 (status, created_at)，索引即有序，免 filesort
ALTER TABLE orders ADD INDEX idx_status_ctime (status, created_at);
SELECT * FROM orders WHERE status = 1 ORDER BY created_at DESC LIMIT 10;
-- 优化前 type=ALL、Extra=Using filesort；优化后 type=ref、无 filesort

-- 慢 SQL 2：改前缀匹配（可用索引）；业务不允许时上全文索引 / ES
SELECT * FROM users WHERE name LIKE '张%';
-- 左模糊 %张% 无法用 B+ 树：前缀未知无法定位起始位置

-- 慢 SQL 3：条件传数值类型，避免隐式类型转换
SELECT * FROM orders WHERE user_id = 12345;
-- 字符串 '12345' 会触发列上的隐式类型转换 → 索引失效（全表扫）

-- 慢 SQL 4：覆盖索引 + 只取必要列；统计类需求拆聚合表
ALTER TABLE orders ADD INDEX idx_ctime_amount (created_at, amount);
SELECT id, amount FROM orders
WHERE created_at BETWEEN '2024-01-01' AND '2024-06-30';
-- Extra 显示 Using index，免回表

-- 慢 SQL 5：游标分页（记住上一页最后一个 id），扫描量从 10 万降到 20
SELECT * FROM orders WHERE id > 100020 ORDER BY id LIMIT 20;
-- 备选：延迟关联（先只查 id 再 join 回全行），适用于需要全列且无法用游标的场景
