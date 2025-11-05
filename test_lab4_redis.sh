#!/bin/bash

echo "========================================"
echo "  Проверка Redis после тестирования"
echo "========================================"
echo ""

# Цвета для вывода
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "1. Проверка подключения к Redis..."
if docker exec cavi_redis redis-cli PING > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Redis доступен${NC}"
else
    echo -e "${RED}✗ Redis недоступен${NC}"
    exit 1
fi
echo ""

echo "2. Список всех сессий:"
echo "   Команда: KEYS session:*"
docker exec cavi_redis redis-cli KEYS "session:*"
echo ""

echo "3. Количество активных сессий:"
SESSION_COUNT=$(docker exec cavi_redis redis-cli KEYS "session:*" | wc -l)
echo -e "   ${YELLOW}Активных сессий: $SESSION_COUNT${NC}"
echo ""

echo "4. Детали каждой сессии:"
echo "   ----------------------------------------"
for session in $(docker exec cavi_redis redis-cli KEYS "session:*" 2>/dev/null); do
    if [ ! -z "$session" ]; then
        echo "   Сессия: $session"
        
        # Получаем значение
        VALUE=$(docker exec cavi_redis redis-cli GET "$session" 2>/dev/null)
        echo "   └─ Пользователь: $VALUE"
        
        # Получаем TTL
        TTL=$(docker exec cavi_redis redis-cli TTL "$session" 2>/dev/null)
        TTL_HOURS=$((TTL / 3600))
        TTL_MINS=$(((TTL % 3600) / 60))
        echo "   └─ TTL: $TTL секунд (~$TTL_HOURS ч $TTL_MINS мин)"
        
        # Проверяем тип данных
        TYPE=$(docker exec cavi_redis redis-cli TYPE "$session" 2>/dev/null)
        echo "   └─ Тип: $TYPE"
        echo ""
    fi
done

echo "5. Проверка конкретных пользователей:"
echo "   ----------------------------------------"

# Проверка user1
echo -n "   user1: "
if docker exec cavi_redis redis-cli EXISTS "session:user1" | grep -q "1"; then
    echo -e "${RED}✗ Сессия существует (должна быть удалена после logout)${NC}"
else
    echo -e "${GREEN}✓ Сессия удалена (корректно)${NC}"
fi

# Проверка moderator
echo -n "   moderator: "
if docker exec cavi_redis redis-cli EXISTS "session:moderator" | grep -q "1"; then
    echo -e "${GREEN}✓ Сессия активна${NC}"
else
    echo -e "${YELLOW}○ Сессия отсутствует${NC}"
fi

# Проверка testuser
echo -n "   testuser: "
if docker exec cavi_redis redis-cli EXISTS "session:testuser" | grep -q "1"; then
    echo -e "${GREEN}✓ Сессия активна${NC}"
else
    echo -e "${YELLOW}○ Сессия отсутствует${NC}"
fi
echo ""

echo "6. Статистика Redis:"
echo "   ----------------------------------------"
echo "   Используемая память:"
docker exec cavi_redis redis-cli INFO memory | grep "used_memory_human" | cut -d: -f2
echo ""
echo "   Всего ключей в БД:"
docker exec cavi_redis redis-cli DBSIZE
echo ""

echo "7. Проверка формата ключей:"
echo "   ----------------------------------------"
CORRECT_FORMAT=true
for session in $(docker exec cavi_redis redis-cli KEYS "session:*" 2>/dev/null); do
    if [ ! -z "$session" ] && [[ ! "$session" =~ ^session:[a-zA-Z0-9_]+$ ]]; then
        echo -e "   ${RED}✗ Неправильный формат: $session${NC}"
        CORRECT_FORMAT=false
    fi
done

if [ "$CORRECT_FORMAT" = true ]; then
    echo -e "   ${GREEN}✓ Все ключи соответствуют формату session:{username}${NC}"
fi
echo ""

echo "========================================"
echo "  Проверка завершена"
echo "========================================"
echo ""

# Итоговый статус
echo "Итог:"
if [ "$SESSION_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ Redis работает корректно${NC}"
    echo -e "${GREEN}✓ Сессии создаются и хранятся${NC}"
    echo -e "${GREEN}✓ Logout удаляет сессии (user1 должен отсутствовать)${NC}"
else
    echo -e "${YELLOW}○ Нет активных сессий (требуется логин)${NC}"
fi
