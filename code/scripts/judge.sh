#!/bin/bash
# GoCampus 一键判题脚本
# 用法：bash scripts/judge.sh

set -e

cd "$(dirname "$0")/.."

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=============================="
echo "  GoCampus 习题判题系统"
echo "=============================="
echo ""

total_pass=0
total_fail=0
total_skip=0

modules=(
    "01_slice"
    "02_map"
    "03_interface"
    "04_goroutine"
    "05_channel"
    "06_sync"
    "07_context"
    "08_memory"
    "09_generics"
    "10_engineering"
)

for module in "${modules[@]}"; do
    if [ ! -d "$module" ]; then
        continue
    fi

    module_pass=0
    module_fail=0
    module_skip=0

    echo -e "${YELLOW}[$module]${NC}"

    for exercise in "$module"/*/; do
        if [ ! -f "$exercise/solution_test.go" ]; then
            continue
        fi

        name=$(basename "$exercise")

        # 检查是否还是 panic("not implemented")
        if grep -q 'panic("not implemented")' "$exercise/solution.go" 2>/dev/null; then
            echo -e "  ${YELLOW}⏭ $name (未实现)${NC}"
            ((module_skip++))
            ((total_skip++))
            continue
        fi

        # 运行测试
        output=$(cd "$exercise" && go test -timeout 10s 2>&1)
        if [ $? -eq 0 ]; then
            echo -e "  ${GREEN}✓ $name${NC}"
            ((module_pass++))
            ((total_pass++))
        else
            echo -e "  ${RED}✗ $name${NC}"
            # 显示失败的测试
            echo "$output" | grep -E "^--- FAIL" | head -3 | sed 's/^/    /'
            ((module_fail++))
            ((total_fail++))
        fi
    done

    echo "  小计: ✓${module_pass} ✗${module_fail} ⏭${module_skip}"
    echo ""
done

echo "=============================="
echo -e "总计: ${GREEN}✓通过 ${total_pass}${NC}  ${RED}✗失败 ${total_fail}${NC}  ${YELLOW}⏭未做 ${total_skip}${NC}"
total=$((total_pass + total_fail + total_skip))
if [ $total -gt 0 ]; then
    pct=$((total_pass * 100 / total))
    echo "完成率: ${pct}% ($total_pass/$total)"
fi
echo "=============================="
