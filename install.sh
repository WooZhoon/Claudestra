#!/bin/bash
set -e

echo "=== Claudestra 빌드 시작 ==="

# Docker 빌드 & 바이너리 추출
mkdir -p out
docker compose run --rm build

# ~/.local/bin에 설치
mkdir -p ~/.local/bin
cp out/claudestra-gui out/claudestra ~/.local/bin/
chmod +x ~/.local/bin/claudestra-gui ~/.local/bin/claudestra

echo ""
echo "=== 설치 완료 ==="
echo ""
echo "  claudestra-gui  → ~/.local/bin/claudestra-gui"
echo "  claudestra      → ~/.local/bin/claudestra"
echo ""

# PATH 확인
if ! echo "$PATH" | grep -q "$HOME/.local/bin"; then
    echo "⚠️  ~/.local/bin이 PATH에 없습니다. 아래를 셸 설정에 추가하세요:"
    echo '  export PATH="$HOME/.local/bin:$PATH"'
    echo ""
fi

echo "실행: claudestra-gui"
