#!/usr/bin/env bash
# 初始化 MySQL 主从复制（GTID + 半同步）
# 用法：bash scripts/init-replication.sh
# 对应文档：docs/架构师修炼/18-实验手册-Go落地实验.md（E02 / E03 / E04）
set -euo pipefail

MASTER="arch-mysql-master"
REPLICA="arch-mysql-replica"
PASS="root123"

echo "==> 等待主从容器就绪"
for c in "$MASTER" "$REPLICA"; do
  until docker exec "$c" mysqladmin ping -uroot -p"$PASS" --silent >/dev/null 2>&1; do
    printf '    waiting for %s ...\n' "$c"
    sleep 2
  done
done

echo "==> 确认主库 GTID 已开启"
docker exec "$MASTER" mysql -uroot -p"$PASS" -N -e \
  "SELECT CONCAT('gtid_mode=', @@gtid_mode, ' binlog_format=', @@binlog_format)"

echo "==> 确认复制账号存在（由 master-init.sql 创建）"
docker exec "$MASTER" mysql -uroot -p"$PASS" -N -e \
  "SELECT CONCAT('repl user: ', user, '@', host) FROM mysql.user WHERE user='repl'"

echo "==> 在从库上配置并启动复制（GTID 自动定位）"
docker exec "$REPLICA" mysql -uroot -p"$PASS" -e "
  STOP REPLICA;
  RESET REPLICA ALL;
  CHANGE REPLICATION SOURCE TO
    SOURCE_HOST='mysql-master',
    SOURCE_PORT=3306,
    SOURCE_USER='repl',
    SOURCE_PASSWORD='repl123',
    SOURCE_AUTO_POSITION=1,
    GET_SOURCE_PUBLIC_KEY=1;
  START REPLICA;
"

sleep 2
echo "==> 复制状态（关注 Replica_IO_Running / Replica_SQL_Running / Seconds_Behind_Source / 两个 GTID 集合）"
docker exec "$REPLICA" mysql -uroot -p"$PASS" -e "SHOW REPLICA STATUS\G" \
  | grep -E "Replica_IO_Running:|Replica_SQL_Running:|Seconds_Behind_Source:|Retrieved_Gtid_Set:|Executed_Gtid_Set:|Last_IO_Error:|Last_SQL_Error:"

echo "==> 复制线程状态（MySQL 8 的 SHOW REPLICA STATUS 别名，MySQL 5.7 用 SHOW SLAVE STATUS）"
cat <<'TIP'

提示：
  1) 主从延迟观察（E02）：
     docker exec -it arch-mysql-replica mysql -uroot -proot123 -e "SHOW REPLICA STATUS\G" | grep -E "Seconds_Behind|Gtid_Set"
     然后在主库跑大事务：
     docker exec -it arch-mysql-master mysql -uroot -proot123 demo -e "UPDATE big_table SET status=1 WHERE id<200000;"
  2) 半同步开关（E04）：
     主库 SET GLOBAL rpl_semi_sync_master_enabled = ON/OFF;
     从库 SET GLOBAL rpl_semi_sync_slave_enabled = OFF;  -- 模拟从库不应答
     观察 SHOW STATUS LIKE 'Rpl_semi_sync_master_status';  -- OFF 说明已静默降级为异步
  3) 主库断电模拟：
     docker kill -s KILL arch-mysql-master
TIP
