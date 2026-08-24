# 一条 SELECT 到底按什么顺序执行？—— SQL 语法详解

> 属于 S1 MySQL 深入 · 基础篇第 2 章
> 上一篇：[MySQL 入门与库表操作](./基础-MySQL入门与库表操作)　下一篇：[表设计与约束范式](./基础-表设计与约束范式)

工作里 80% 的时间在写查询。很多同学能"查出来结果"，但一被问"`WHERE` 和 `HAVING` 谁先执行""JOIN 和子查询哪个快"就露怯。这一篇把 DQL 的执行顺序、JOIN、子查询、窗口函数一次讲透，全部配可直接执行的示例。

---

## SELECT 的书写顺序 ≠ 执行顺序

这是基础篇最重要的一个知识点，面试必问。SQL 的**书写顺序**和**逻辑执行顺序**不一样：

```text
书写顺序：  SELECT → FROM → WHERE → GROUP BY → HAVING → ORDER BY → LIMIT
逻辑顺序：  FROM → WHERE → GROUP BY → HAVING → SELECT → DISTINCT → ORDER BY → LIMIT
```

执行顺序的通俗理解：先确定"从哪张表取"（FROM），再按条件"筛行"（WHERE），然后"分组"（GROUP BY）并"筛组"（HAVING），最后才"选列"（SELECT，此时才能用别名）、"排序"（ORDER BY）、"取前 N 条"（LIMIT）。

这个顺序解释了很多"反直觉"现象，比如：

- `WHERE` 里**不能用 SELECT 的别名**（`WHERE age > 18` 可以，`WHERE age_plus > 18` 不行——WHERE 在 SELECT 之前执行，别名还不存在）；但 `ORDER BY` 可以用别名（它在 SELECT 之后执行）。
- `HAVING` 能用聚合结果（`HAVING COUNT(*) > 10`），`WHERE` 不能（WHERE 在分组前，此时还没有聚合值）。

## WHERE 过滤与运算符

```sql
-- 从用户表里筛出成年用户
SELECT id, name, age FROM users WHERE age >= 18;

-- 组合条件：AND 优先级高于 OR，需要括号显式分组
SELECT * FROM users WHERE (age >= 18 AND gender = 1) OR vip = 1;

-- 模糊匹配：% 任意多个字符，_ 单个字符
SELECT * FROM users WHERE name LIKE '张%';   -- 张开头
SELECT * FROM users WHERE name LIKE '张_';   -- 张+1个字

-- 范围与集合
SELECT * FROM users WHERE age BETWEEN 18 AND 30;
SELECT * FROM users WHERE id IN (1, 2, 3);
SELECT * FROM users WHERE email IS NOT NULL;   -- 判空用 IS NULL / IS NOT NULL，不能写 = NULL

-- NULL 的坑：NULL 参与任何比较结果都是 NULL（既不是 true 也不是 false）
SELECT * FROM users WHERE age = NULL;  -- 永远查不到，必须写 IS NULL
```

## 聚合与分组：GROUP BY / HAVING

聚合函数把多行压成一行：`COUNT`（计数）、`SUM`（求和）、`AVG`（平均）、`MAX`/`MIN`（最大/最小）。

```sql
-- 每个城市的用户数，且只看用户数 > 100 的城市
SELECT city, COUNT(*) AS cnt
FROM users
WHERE status = 1                    -- 1) 先筛行
GROUP BY city                       -- 2) 再分组
HAVING COUNT(*) > 100               -- 3) 再筛组（不能写 cnt > 100？可以但部分版本/模式受限，标准写法用聚合函数）
ORDER BY cnt DESC                   -- 4) 排序
LIMIT 10;                           -- 5) 取前 10
```

**记住**：`WHERE` 筛的是"行"，`HAVING` 筛的是"组"。`WHERE` 在分组前执行、不能用聚合函数；`HAVING` 在分组后执行、可以用聚合函数。能用 `WHERE` 提前筛掉的尽量用 `WHERE`（`HAVING` 处理的是已分组的行，性能更差）。

## JOIN：五类连接一张表讲清

JOIN 把两张表按条件"拼"起来。先建两张演示表：

```sql
-- users(id, name, dept_id)  departments(id, name)
```

| JOIN 类型 | 结果 | 通俗理解 |
|-----------|------|----------|
| `INNER JOIN` | 两表都匹配的行 | 交集 |
| `LEFT JOIN` | 左表全部 + 右表匹配的 | 左表全保留，右表没匹配就补 NULL |
| `RIGHT JOIN` | 右表全部 + 左表匹配的 | 右表全保留（日常更常用 LEFT，把 RIGHT 翻转写） |
| `CROSS JOIN` | 笛卡尔积 | 两表行数相乘，一般配合条件用 |
| 自连接（self join） | 表和自身连接 | 如"查同一部门的人"，用别名区分两份 |

```sql
-- LEFT JOIN：查出所有用户及其部门名（没有部门的用户，dept_name 为 NULL）
SELECT u.id, u.name, d.name AS dept_name
FROM users u
LEFT JOIN departments d ON u.dept_id = d.id;

-- 用 LEFT JOIN + IS NULL 实现"不在 B 表"的经典写法（替代 NOT IN，性能更稳）
SELECT u.id, u.name
FROM users u
LEFT JOIN departments d ON u.dept_id = d.id
WHERE d.id IS NULL;                  -- 没有部门（dept_id 在 departments 里不存在）的用户
```

> **面试高频**：`LEFT JOIN` 的 ON 条件里加 `d.status = 1` 和 WHERE 里加 `d.status = 1` 有什么区别？—— ON 里的条件是"连接时过滤"，右表不匹配的仍会保留为 NULL 行；WHERE 里的条件是"连接后再过滤"，会把 NULL 行直接筛掉，效果接近 INNER JOIN。**想保留左表全量，过滤右表的条件要写在 ON 里**。

## 子查询：标量、行、表

子查询就是"查询里的查询"，按返回内容分三类：

```sql
-- 1) 标量子查询：返回单个值，可用在 SELECT / WHERE 里
SELECT name FROM users
WHERE dept_id = (SELECT id FROM departments WHERE name = 'AI 部门');

-- 2) 行子查询：返回一行
SELECT * FROM users
WHERE (dept_id, age) = (SELECT dept_id, MAX(age) FROM users GROUP BY dept_id LIMIT 1);

-- 3) 表子查询：返回多行多列，可用在 FROM（派生表）或 IN / EXISTS 里
SELECT d.name, t.cnt
FROM departments d
JOIN (SELECT dept_id, COUNT(*) AS cnt FROM users GROUP BY dept_id) t
  ON d.id = t.dept_id;

-- IN 与 EXISTS 的语义区别（重要）：
-- IN：先执行子查询拿到集合，再外层匹配（子查询结果大时浪费）
SELECT * FROM users WHERE dept_id IN (SELECT id FROM departments WHERE name LIKE 'AI%');
-- EXISTS：外层逐行判断子查询是否有结果，命中即停（适合子查询大、外层小的场景）
SELECT * FROM users u
WHERE EXISTS (SELECT 1 FROM departments d WHERE d.id = u.dept_id AND d.name LIKE 'AI%');
```

> 8.0 优化器已经会做子查询改写（把 IN 改写成半连接 semi-join），所以性能差异没 5.7 时代那么大，但**语义差异必须懂**：`IN` 返回集合，`EXISTS` 只关心"有没有"（所以子查询里写 `SELECT 1` 而不是 `SELECT *`）。

## 窗口函数：不丢行的"分组计算"

窗口函数（8.0+）和 GROUP BY 最大的区别：**GROUP BY 把多行压成一行，窗口函数保留每一行，在旁边"开一扇窗"计算**。语法：`函数() OVER (PARTITION BY 分组 ORDER BY 排序)`。

```sql
-- 每个部门内按工资排名：ROW_NUMBER 无并列 / RANK 有并列跳号 / DENSE_RANK 有并列不跳号
SELECT name, dept_id, salary,
       ROW_NUMBER() OVER (PARTITION BY dept_id ORDER BY salary DESC) AS rn,
       RANK()       OVER (PARTITION BY dept_id ORDER BY salary DESC) AS rk,
       DENSE_RANK() OVER (PARTITION BY dept_id ORDER BY salary DESC) AS drk
FROM employees;

-- 取每个部门工资最高的前 2 名（经典 Top-N，子查询包一层）
SELECT * FROM (
  SELECT name, dept_id, salary,
         ROW_NUMBER() OVER (PARTITION BY dept_id ORDER BY salary DESC) AS rn
  FROM employees
) t WHERE t.rn <= 2;

-- 累计/移动计算：SUM OVER 累加，LAG/LEAD 取前一行/后一行
SELECT day, amount,
       SUM(amount) OVER (ORDER BY day) AS running_total,   -- 累计
       LAG(amount, 1) OVER (ORDER BY day) AS prev_amount   -- 前一天金额（同比/环比用）
FROM sales;
```

窗口函数是"分组 Top-N、排名、累计、环比"这些面试场景题的标准答案，**能用窗口函数就不要写自连接**。

## DML：增删改 + 事务注意

```sql
-- 插入：指定列（推荐）vs 全列
INSERT INTO users (name, email, age) VALUES ('张三', 'zs@example.com', 25);
INSERT INTO users (name, email) VALUES ('李四', 'ls@example.com'), ('王五', 'ww@example.com'); -- 批量

-- 更新：不加 WHERE = 全表更新！先 SELECT 确认再 UPDATE 是铁律
UPDATE users SET age = 26 WHERE name = '张三';

-- 删除：同上，WHERE 必须有
DELETE FROM users WHERE id = 10086;

-- 不存在则插入、存在则更新（8.0 语法，幂等写入的常用招）
INSERT INTO users (id, name, email) VALUES (1, '张三', 'zs@example.com')
ON DUPLICATE KEY UPDATE name = VALUES(name);
```

> **DML 与事务**：INSERT/UPDATE/DELETE 默认是"自动提交"的（autocommit=1，每条语句立即生效）。要让它可回滚，必须显式 `START TRANSACTION` / `COMMIT` / `ROLLBACK` —— 下一篇《表设计与约束范式》讲完表设计，《事务与索引入门》会专门讲事务。

## 常用函数速查（够用版）

| 类别 | 常用函数 |
|------|----------|
| 字符串 | `CONCAT`（拼接）、`SUBSTRING`（截取）、`LENGTH`/`CHAR_LENGTH`（字节/字符长度）、`UPPER`/`LOWER`、`TRIM`、`REPLACE` |
| 日期 | `NOW()`/`CURRENT_TIMESTAMP`、`DATE_FORMAT(d, '%Y-%m-%d')`、`DATEDIFF`（天数差）、`DATE_ADD(d, INTERVAL 1 DAY)` |
| 数值 | `ROUND`、`FLOOR`、`CEIL`、`ABS`、`MOD` |
| 条件 | `IF(cond, a, b)`、`CASE WHEN ... THEN ... ELSE ... END`、`IFNULL(a, b)`（NULL 兜底） |
| 聚合 | `COUNT`/`SUM`/`AVG`/`MAX`/`MIN`（配 `DISTINCT` 去重：`COUNT(DISTINCT city)`） |

```sql
-- CASE WHEN 是"行转列/分段统计"的基础（面试场景题常用）
SELECT
  SUM(CASE WHEN age < 18 THEN 1 ELSE 0 END) AS minors,
  SUM(CASE WHEN age BETWEEN 18 AND 60 THEN 1 ELSE 0 END) AS adults
FROM users;
```

> **函数与索引的坑**：对索引列使用函数（如 `WHERE DATE_FORMAT(created_at, '%Y-%m-%d') = '2024-01-01'`）会导致索引失效——这个坑在深入篇《索引与 SQL 优化》会重点讲。

---

## 串起来

查询这一关，抓住一条主线就够：**SQL 的执行顺序 FROM → WHERE → GROUP BY → HAVING → SELECT → ORDER BY → LIMIT**，它能解释别名、HAVING、排序的一切怪现象。JOIN 记住"LEFT JOIN 保留左表、过滤右表写 ON 里"，窗口函数记住"不压行、分组 Top-N 全靠它"。剩下的就是多写。

下一篇进入 **表设计与约束范式**：字段类型怎么选、约束怎么用、三范式要不要全守，把"建表"这件事做扎实。
