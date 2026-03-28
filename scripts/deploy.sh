#!/bin/bash
set -euo pipefail

# FastClaw 部署脚本
# 用法：./scripts/deploy.sh

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }
step()  { echo -e "${BLUE}[STEP]${NC} $*"; }

echo ""
echo "  ⚡ FastClaw 部署脚本"
echo "  ====================="
echo ""

# 步骤 1: 检查 Go 环境
step "检查 Go 环境..."
if ! command -v go &>/dev/null; then
    error "Go 未安装！请先安装 Go 1.25+: https://go.dev/dl/"
fi

GO_VERSION=$(go version)
info "Go 环境：$GO_VERSION"

# 检查 Go 版本 (需要 1.25+)
GO_VER_NUM=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1 | sed 's/go//')
MAJOR=$(echo "$GO_VER_NUM" | cut -d. -f1)
MINOR=$(echo "$GO_VER_NUM" | cut -d. -f2)

if [ "$MAJOR" -lt 1 ] || ([ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 25 ]); then
    warn "Go 版本可能过低 (需要 1.25+)，当前：$GO_VER_NUM"
fi

# 步骤 2: 检查 Node.js 和 pnpm (用于构建 Web UI)
step "检查前端构建工具..."
if ! command -v node &>/dev/null; then
    warn "Node.js 未安装，将无法构建 Web UI"
    warn "如需完整功能，请安装 Node.js: https://nodejs.org/"
    SKIP_WEB=true
else
    info "Node.js: $(node --version)"
    if command -v pnpm &>/dev/null; then
        info "pnpm: $(pnpm --version)"
    else
        warn "pnpm 未安装，尝试使用 npm 安装..."
        npm install -g pnpm 2>/dev/null || warn "pnpm 安装失败，将跳过 Web UI 构建"
        SKIP_WEB=false
    fi
fi

# 步骤 3: 创建目录
step "创建必要的目录..."
mkdir -p bin
mkdir -p ~/.fastclaw
info "已创建 bin/ 和 ~/.fastclaw 目录"

# 步骤 4: 构建 Web UI (如果可能)
if [ "${SKIP_WEB:-false}" != "true" ]; then
    step "构建 Web UI..."
    if [ -d "web" ] && [ -f "web/pnpm-lock.yaml" ]; then
        cd web
        pnpm install --frozen-lockfile 2>/dev/null || pnpm install
        pnpm build || {
            warn "Web UI 构建失败，将继续构建纯后端版本"
            SKIP_WEB=true
        }
        cd ..
        
        if [ "$SKIP_WEB" != "true" ]; then
            rm -rf internal/setup/web 2>/dev/null || true
            cp -r web/out internal/setup/web 2>/dev/null || warn "复制 Web UI 失败"
            info "Web UI 构建完成"
        fi
    else
        warn "web 目录不完整，跳过 Web UI 构建"
        SKIP_WEB=true
    fi
else
    warn "跳过 Web UI 构建"
fi

# 步骤 5: 构建 Go 二进制
step "构建 FastClaw 二进制文件..."
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE"

CGO_ENABLED=0 go build -ldflags "$LDFLAGS" -o bin/fastclaw ./cmd/fastclaw

if [ -f "bin/fastclaw" ]; then
    chmod +x bin/fastclaw
    info "✅ 构建成功：bin/fastclaw"
    ls -lh bin/fastclaw
else
    error "构建失败！"
fi

# 步骤 6: 创建默认配置
step "检查配置文件..."
CONFIG_FILE="$HOME/.fastclaw/fastclaw.json"
if [ ! -f "$CONFIG_FILE" ]; then
    info "创建默认配置文件..."
    cat > "$CONFIG_FILE" << 'CFGEOF'
{
  "gateway": {
    "port": 18953,
    "bind": "loopback",
    "auth": {
      "mode": "token",
      "token": ""
    },
    "http": {
      "endpoints": {
        "chatCompletions": {
          "enabled": true
        }
      }
    }
  },
  "providers": {},
  "agents": {
    "defaults": {
      "model": "",
      "maxTokens": 8192,
      "temperature": 0.7,
      "maxToolIterations": 20
    },
    "list": []
  },
  "channels": {},
  "storage": {
    "type": "file",
    "path": "~/.fastclaw/data"
  }
}
CFGEOF
    info "配置文件已创建：$CONFIG_FILE"
    warn "请编辑配置文件添加你的 LLM API 密钥"
else
    info "配置文件已存在：$CONFIG_FILE"
fi

# 步骤 7: 创建示例 agent 目录
AGENT_DIR="$HOME/.fastclaw/agents/main"
if [ ! -d "$AGENT_DIR" ]; then
    mkdir -p "$AGENT_DIR"
    cat > "$AGENT_DIR/SOUL.md" << 'SOULEOF'
# Agent: main

你是一个有帮助的 AI 助手。

## 能力
- 回答问题
- 执行工具调用
- 管理文件和记忆

## 风格
- 友好、专业
- 简洁明了
SOULEOF
    info "已创建示例 Agent: $AGENT_DIR"
fi

# 步骤 8: 验证
step "验证部署..."
./bin/fastclaw version

echo ""
echo "  ========================================"
echo "  ✅ FastClaw 部署完成！"
echo "  ========================================"
echo ""
echo "  下一步操作:"
echo ""
echo "  1. 编辑配置文件添加 LLM API 密钥:"
echo "     nano $CONFIG_FILE"
echo ""
echo "  2. 启动 FastClaw:"
echo "     ./bin/fastclaw"
echo "     或安装到系统路径后运行:"
echo "     sudo cp bin/fastclaw /usr/local/bin/"
echo "     fastclaw"
echo ""
echo "  3. 访问 Web 界面:"
echo "     http://localhost:18953"
echo ""
echo "  常用命令:"
echo "     fastclaw version    # 查看版本"
echo "     fastclaw doctor     # 健康检查"
echo "     fastclaw upgrade    # 升级"
echo "     fastclaw --help     # 帮助"
echo ""
