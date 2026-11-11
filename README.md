# DolphinScheduler Console for DBX

DBX 插件：管理 DolphinScheduler 项目、工作流、运行实例、定时调度和任务日志，提供 31 个 MCP 工具。

**连接由每个用户自行配置。安装包不含任何 DS 地址、账号、Token 或内网配置。**

## 本地安装（Windows x64）

1. 使用支持插件 MCP contribution 的新版 DBX（清单最低版本为 0.5.68）。
2. 在插件中心设置中开启 **Allow unsigned development packages / 允许未签名开发包**。
3. 使用插件中心的本地安装入口选择 `.dbxp` 文件，安装并启用插件。
4. 新建 **DolphinScheduler 连接**，填入以下配置，点击测试，保存并打开。

| 配置项 | 填写方式 |
| --- | --- |
| 连接名称 | 自行命名，例如测试环境、生产只读 |
| API 主机 | 自己的域名或 IP；不含协议和路径 |
| API 端口 | 服务或反向代理端口，默认 12345；HTTPS 反向代理通常需改为 443 |
| 协议 | HTTP 或 HTTPS |
| API 根路径 | 默认 `/dolphinscheduler`；按实际反向代理路径修改，根路径服务填 `/` |
| API Token | 用户在自己的 DolphinScheduler 中创建的有效访问令牌 |
| 只读连接 | 默认开启；关闭后需重新连接才能使用修改操作 |
| 请求超时 | 1–120 秒，默认 30 秒 |

Token 使用 DBX 的 `binding: secret` 密钥存储，只传给后端。工作台不接收 Token。
可新建多个连接分别保存不同环境。连接测试会访问项目分页接口，因此 Token 需要相应读取权限。

## 使用

- **项目**：选择项目，或直接填写完整的项目 Code。
- **工作流定义**：查询定义、节点和版本；允许写入时可创建、更新、发布和运行。
- **运行实例**：查看实例、任务及日志；允许写入时可停止、暂停、重跑等。
- **定时调度**：查询、创建、修改和上下线；全部工具中也提供删除操作。
- **任务日志**：填写任务实例 ID、起始行和读取行数。
- **全部工具**：使用根据参数 schema 生成的表单，支持全部 31 个操作。

所有修改操作都要求 `confirm=true`，且只读连接在后端强制拒绝修改。生成任务 Code 的 GET 接口也按写入操作管理。
64 位项目/工作流 Code 使用十进制字符串，超出 JavaScript 安全整数范围的响应值转换成字符串，避免精度丢失。

## AI / MCP

- DBX 内置 AI：在插件中心为该插件开启内置 AI 工具，并打开目标连接。
- 外部 MCP：在 DBX 设置的 MCP 页面允许该插件与目标连接，选择合适的读写权限。
- 工具声明分为 `read`、`write`、`full`。启动、停止、发布、上下线等使用 `full`。
- DBX 宿主绑定连接并注入凭据；工具不接受地址或 Token 参数。

## 验证与兼容范围

基于提供的 DolphinScheduler OpenAPI 文档（标注 3.3.0，`process-definition` / `process-instances` 路由系列）实现。
已通过 Go 模拟 HTTP 测试、官方 DBX 开发宿主与 Chromium 端到端测试；**尚未连接用户真实 DS 环境验证**。
其他版本和企业定制接口需实际联调；使用 `workflow-definition` 路由的新接口版本不在当前适配范围内。

当前交付 Windows x64。支持 HTTP/HTTPS 和 DBX 静态 SSH 端口转发；HTTPS 校验系统证书，不提供跳过校验开关。
暂不支持 SOCKS5 动态路由、文件上传下载、用户/租户管理和可视化 DAG 编辑。
项目下拉框加载前 100 个可见项目；其他项目可直接输入 Code，项目列表支持分页。

## 开发

需要 Go 1.22+、Node.js 22+ 和官方 `@dbx-app/plugin-cli@0.1.10`。后端使用 Go 标准库及随源码附带的官方 Go SDK，不需 Python 运行时。

```sh
cd backend
go test ./...
go vet ./...
cd ..
npx @dbx-app/plugin-cli@0.1.10 dev --path .
npx @dbx-app/plugin-cli@0.1.10 package .
```

Python 仅用于开发时校验与从外部 OpenAPI 文档重新生成工具；不随插件运行。

```sh
python -m pip install jsonschema==4.23.0
python scripts/validate.py
python generate.py /path/to/your/openapi.json
```

`generate.py` 只提取审核过的操作及参数，不复制服务器地址或配置。
DBX 开发宿主可能在 `.dbx-dev` 中保存明文测试配置，请只用测试数据且不要提交该目录。

## 发布与商店

- `.github/workflows/ci.yml`：Linux / Windows 自动测试、清单与工具契约校验。
- `.github/workflows/release.yml`：发布 GitHub Release 后构建 Windows 安装包、候选元数据和每个平台的 artifact 文件。
- [中文商店上架指南](docs/MARKETPLACE.zh-CN.md)
- [验证记录](docs/VALIDATION.md)

GitHub 发布使用自带的 `GITHUB_TOKEN`，不需要在仓库配置 DS 凭据、PAT 或商店签名私钥。
当前安装包是未签名开发候选包。官方商店包由 DBX Store 审核并签名后发布。

## 许可证

Apache-2.0。官方 DBX SDK 的来源与许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
