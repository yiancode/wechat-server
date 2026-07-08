#!/bin/bash
#
# wechat-server 零停机部署脚本（参考 code80 部署脚本结构）
# 目标: 106.53.117.99:/opt/wechat-server
# 本地交叉编译 → 上传到临时位置 → 原子替换二进制，停机时间仅几秒
# 服务管理: systemd (wechat-server.service)
#
# 注意: 服务器上的 /opt/wechat-server/config.yaml 含真实密钥且以服务器为准，
#       本脚本【不上传、不覆盖】配置文件；改配置请直接在服务器上编辑后重启。
#

set -e

# ============================================================
# 配置区域
# ============================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 默认配置（由 scripts/deploy.conf 覆盖）
REMOTE_HOST="106.53.117.99"
REMOTE_USER="root"
REMOTE_DIR="/opt/wechat-server"
BINARY_NAME="wechat-server"
SSH_KEY=""
SERVICE_NAME="wechat-server"

# 服务端口（部署后用于验证 listen 是否成功；留空则跳过端口校验）
SERVICE_PORT="3000"
SERVICE_PORT_WAIT_SECONDS=30

# 加载本地覆盖配置
CONF_FILE="$SCRIPT_DIR/deploy.conf"
if [ -f "$CONF_FILE" ]; then
    source "$CONF_FILE"
fi

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error()   { echo -e "${RED}[ERROR]${NC} $1"; }

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        print_error "未找到命令: $1"
        exit 1
    fi
}

# SSH/SCP 命令封装（支持自定义密钥）
ssh_cmd() {
    local expanded_key="${SSH_KEY/#\~/$HOME}"
    local -a ssh_opts=(
        -o ConnectTimeout=10
        -o ServerAliveInterval=15
        -o ServerAliveCountMax=6
    )
    if [ -n "$expanded_key" ] && [ -f "$expanded_key" ]; then
        ssh "${ssh_opts[@]}" -i "$expanded_key" "${REMOTE_USER}@${REMOTE_HOST}" "$@"
    else
        ssh "${ssh_opts[@]}" "${REMOTE_USER}@${REMOTE_HOST}" "$@"
    fi
}

run_scp() {
    local use_legacy="$1"
    shift
    local expanded_key="${SSH_KEY/#\~/$HOME}"
    local -a scp_opts=(
        -o ConnectTimeout=10
        -o ServerAliveInterval=15
        -o ServerAliveCountMax=6
    )
    if [ "$use_legacy" = true ]; then
        scp_opts=(-O "${scp_opts[@]}")
    fi
    if [ -n "$expanded_key" ] && [ -f "$expanded_key" ]; then
        scp "${scp_opts[@]}" -i "$expanded_key" "$@"
    else
        scp "${scp_opts[@]}" "$@"
    fi
}

scp_cmd() {
    local max_attempts=3
    local attempt=1
    local legacy_mode=false
    while [ "$attempt" -le "$max_attempts" ]; do
        [ "$attempt" -ge 2 ] && legacy_mode=true
        if run_scp "$legacy_mode" "$@"; then
            return 0
        fi
        if [ "$legacy_mode" = false ]; then
            print_warning "SCP 默认模式失败，下次将用 -O 兼容模式重试。"
        else
            print_warning "上传失败，准备第 ${attempt}/${max_attempts} 次重试..."
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    return 1
}

# 等待远端端口进入 LISTEN 状态
wait_for_port() {
    local port="$1"
    local max_seconds="${2:-30}"
    local interval=2
    local elapsed=0

    print_info "等待端口 ${port} 监听（最长 ${max_seconds}s，每 ${interval}s 轮询）..."
    while [ "$elapsed" -lt "$max_seconds" ]; do
        if ssh_cmd "ss -tlnp 2>/dev/null | grep -q ':${port} '"; then
            print_success "端口 ${port} 已监听 (耗时 ~${elapsed}s)"
            return 0
        fi
        sleep "$interval"
        elapsed=$((elapsed + interval))
    done

    print_error "端口 ${port} 在 ${max_seconds}s 内未监听 —— 启动失败或卡死"
    return 1
}

# 服务健康校验：优先端口，其次 systemd active 状态
verify_service_up() {
    if [ -n "$SERVICE_PORT" ]; then
        wait_for_port "$SERVICE_PORT" "$SERVICE_PORT_WAIT_SECONDS"
        return $?
    fi
    sleep 3
    ssh_cmd "systemctl is-active --quiet ${SERVICE_NAME}"
}

# ============================================================
# 主流程
# ============================================================
main() {
    echo ""
    echo "=============================================="
    echo "   wechat-server 零停机部署 (原子替换)"
    echo "=============================================="
    echo ""

    SKIP_BUILD=false
    FIRST_DEPLOY=false

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-build)   SKIP_BUILD=true;   shift ;;
            --first-deploy) FIRST_DEPLOY=true; shift ;;
            --help|-h)
                echo "用法: $0 [选项]"
                echo ""
                echo "选项:"
                echo "  --skip-build     跳过编译，直接部署已有的 ${BINARY_NAME}-linux"
                echo "  --first-deploy   首次部署：初始化目录并安装 systemd 服务"
                echo "  -h, --help       显示帮助"
                echo ""
                echo "配置覆盖: 在 scripts/deploy.conf 中定义 REMOTE_HOST/REMOTE_DIR 等变量"
                echo "注意: 本脚本不上传 config.yaml，服务器上的配置（含密钥）以服务器为准"
                exit 0
                ;;
            *)
                print_error "未知参数: $1"
                exit 1
                ;;
        esac
    done

    cd "$PROJECT_ROOT"

    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    print_info "当前分支: $CURRENT_BRANCH ($COMMIT)"
    print_info "项目目录: $PROJECT_ROOT"
    print_info "目标服务器: ${REMOTE_USER}@${REMOTE_HOST}"
    print_info "部署目录: ${REMOTE_DIR}"
    print_info "systemd 服务: ${SERVICE_NAME}"
    echo ""

    require_cmd ssh
    require_cmd scp

    LOCAL_BINARY="${BINARY_NAME}-linux"

    # ── Step 0: 首次部署初始化 ──────────────────────────────
    if [ "$FIRST_DEPLOY" = true ]; then
        print_info "Step 0: 首次部署初始化..."
        ssh_cmd "mkdir -p ${REMOTE_DIR}"
        print_success "目录结构已创建"

        SERVICE_FILE="$PROJECT_ROOT/deploy/wechat-server.service"
        if [ -f "$SERVICE_FILE" ]; then
            scp_cmd "$SERVICE_FILE" "${REMOTE_USER}@${REMOTE_HOST}:/tmp/${SERVICE_NAME}.service"
            ssh_cmd "mv /tmp/${SERVICE_NAME}.service /etc/systemd/system/${SERVICE_NAME}.service && \
                     systemctl daemon-reload && \
                     systemctl enable ${SERVICE_NAME}"
            print_success "systemd 服务已安装并设为开机启动"
        else
            print_warning "未找到 $SERVICE_FILE，请手动安装 systemd 服务"
        fi

        print_warning "首次部署请在服务器上创建配置文件（参考 config.example.yaml）："
        print_warning "  ssh ${REMOTE_USER}@${REMOTE_HOST}"
        print_warning "  vi ${REMOTE_DIR}/config.yaml && chmod 600 ${REMOTE_DIR}/config.yaml"
        echo ""
    fi

    # ── Step 1: 交叉编译 (linux/amd64) ───────────────────────
    if [ "$SKIP_BUILD" = false ]; then
        require_cmd go
        print_info "Step 1/4: 编译 (linux/amd64)..."
        go test ./... >/dev/null
        print_success "单测通过"
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
            go build -buildvcs=false -ldflags="-s -w" -trimpath \
                -o "$LOCAL_BINARY" .
        print_success "编译完成: $LOCAL_BINARY ($(du -sh "$LOCAL_BINARY" | cut -f1))"
    else
        print_warning "Step 1/4: 跳过编译"
    fi

    if [ ! -f "$LOCAL_BINARY" ]; then
        print_error "未找到二进制: $LOCAL_BINARY"
        exit 1
    fi

    # ── Step 2: 上传二进制到临时位置（服务保持运行）──────────
    print_info "Step 2/4: 上传二进制到临时位置..."
    TEMP_BINARY="/tmp/${BINARY_NAME}.new.$$"
    if ! scp_cmd "$LOCAL_BINARY" "${REMOTE_USER}@${REMOTE_HOST}:${TEMP_BINARY}"; then
        print_error "二进制上传失败"
        exit 1
    fi
    ssh_cmd "chmod +x ${TEMP_BINARY}"
    print_success "二进制上传完成: ${TEMP_BINARY}"

    # ── Step 3: 原子替换（停机几秒）─────────────────────────
    print_info "Step 3/4: 执行原子替换（停机约 2-3 秒）..."

    # 备份当前版本（带时间戳，最多保留最近 5 份）
    ssh_cmd "cd ${REMOTE_DIR} && \
        if [ -f ${BINARY_NAME} ]; then \
            cp ${BINARY_NAME} ${BINARY_NAME}.backup; \
            cp ${BINARY_NAME} ${BINARY_NAME}.bak.\$(date +%Y%m%d-%H%M%S); \
            ls -t ${BINARY_NAME}.bak.* 2>/dev/null | tail -n +6 | xargs -r rm -f; \
        fi"

    print_info "停止服务 [${SERVICE_NAME}]..."
    ssh_cmd "systemctl stop ${SERVICE_NAME} || true" 2>&1 | cat
    sleep 1

    ssh_cmd "mv ${TEMP_BINARY} ${REMOTE_DIR}/${BINARY_NAME}"
    print_success "二进制替换完成"

    print_info "启动服务..."
    ssh_cmd "systemctl start ${SERVICE_NAME}" 2>&1 | cat
    sleep 1

    # ── Step 4: 验证 ─────────────────────────────────────────
    print_info "Step 4/4: 验证服务状态..."
    if verify_service_up; then
        echo ""
        ssh_cmd "systemctl status ${SERVICE_NAME} --no-pager -l | head -12" 2>&1 | cat
        echo ""
        ssh_cmd "journalctl -u ${SERVICE_NAME} -n 10 --no-pager" 2>&1 | cat || true
    else
        print_error "服务启动失败！"
        print_info "查看 systemd 日志..."
        ssh_cmd "journalctl -u ${SERVICE_NAME} -n 50 --no-pager" 2>&1 | cat || true

        # 尝试回滚
        print_warning "尝试回滚到备份版本..."
        ssh_cmd "systemctl stop ${SERVICE_NAME} 2>/dev/null || true; \
            cd ${REMOTE_DIR} && \
            [ -f ${BINARY_NAME}.backup ] && mv ${BINARY_NAME}.backup ${BINARY_NAME}; \
            systemctl start ${SERVICE_NAME}" 2>&1 | cat || true

        if verify_service_up; then
            print_warning "已回滚到备份版本，请排查新版本启动问题。"
        else
            print_error "回滚后仍未恢复 —— 需立即人工介入。"
        fi
        exit 1
    fi

    echo ""
    echo "=============================================="
    print_success "wechat-server 部署完成！停机时间约 2-3 秒"
    echo "=============================================="
    echo ""
    echo "  服务器: ${REMOTE_HOST}:${REMOTE_DIR}"
    echo "  服务:   ${SERVICE_NAME} (端口 ${SERVICE_PORT:-见 config.yaml})"
    echo "  版本:   ${CURRENT_BRANCH} @ ${COMMIT}"
    echo ""
    echo "  查看日志: ssh ${REMOTE_USER}@${REMOTE_HOST} 'journalctl -u ${SERVICE_NAME} -f'"
    echo "  回滚:     ssh ${REMOTE_USER}@${REMOTE_HOST} 'cd ${REMOTE_DIR} && systemctl stop ${SERVICE_NAME} && mv ${BINARY_NAME}.backup ${BINARY_NAME} && systemctl start ${SERVICE_NAME}'"
    echo ""
    echo "  提醒: config.yaml 不会被本脚本改动；如需调整 forwarders/headers，"
    echo "        请在服务器上编辑 ${REMOTE_DIR}/config.yaml 后 systemctl restart ${SERVICE_NAME}"
    echo ""
}

main "$@"
