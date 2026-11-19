# dkl4ii67.zjz-service.cn 部署说明

## 当前拓扑

- 公网：`https://dkl4ii67.zjz-service.cn/`
- Nginx：监听宿主机 `8080`，静态站点为 `/opt/go-qianchuan/frontend/dist`
- Go 后端：systemd `qianchuan-backend.service`，监听 `127.0.0.1:18081`
- 访问控制：`/access` 页面使用 `APP_ACCESS_USERNAME` 和 `APP_ACCESS_PASSWORD_SHA256`，Session 保存在服务端内存
- OAuth 回调需要跨站携带访问 Session，因此生产环境使用 `COOKIE_SECURE=true`、`COOKIE_SAME_SITE=none`
- Nginx 将 `/api/`、`/health`、`/ready`、`/metrics` 和 `/swagger/` 转发到 Go 后端
- 旧 `qc-workbench.service` 保留在 `/opt/qc-live-workbench`，切换后已停止并禁用开机自启

## 首次部署

```bash
cd /opt/go-qianchuan/backend
export PATH=/opt/toolchains/go1.24.5/bin:$PATH
export GOPROXY=https://goproxy.cn,direct
go build -trimpath -o bin/qianchuan-server ./cmd/server
go build -trimpath -o bin/qianchuan-mcp ./cmd/qianchuan-mcp

install -m 0755 deploy/zjz-service/qianchuan-mcp.sh /opt/go-qianchuan/backend/bin/qianchuan-mcp.sh

install -m 0644 deploy/zjz-service/qianchuan-backend.service /etc/systemd/system/qianchuan-backend.service
install -m 0644 deploy/zjz-service/go-qianchuan.conf /www/server/panel/vhost/nginx/go-qianchuan.conf
systemctl daemon-reload
systemctl enable --now qianchuan-backend.service
nginx -t
nginx -s reload
```

## MCP 客户端

远端 MCP 客户端通过 SSH 执行：

```text
/opt/go-qianchuan/backend/bin/qianchuan-mcp.sh
```

包装器从 `/opt/go-qianchuan/backend/.env` 读取 `MCP_API_KEY`，以标准输入输出运行 `qianchuan-mcp`。密钥不会出现在 SSH 参数、客户端配置或服务日志中。

## 回滚到旧工作台

```bash
systemctl stop qianchuan-backend.service
systemctl disable qianchuan-backend.service
rm -f /www/server/panel/vhost/nginx/go-qianchuan.conf
rm -f /www/server/panel/vhost/nginx/go-qianchuan-test.conf
nginx -t
nginx -s reload
systemctl enable --now qc-workbench.service
```

旧工作台的数据和程序均未删除。
