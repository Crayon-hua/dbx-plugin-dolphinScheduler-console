# 从本地测试到 DBX 官方商店

先完成真实 DolphinScheduler 环境的本地验收，再提交商店。此仓库不会自动向 DBX Store 提交 PR。

## 1. 验收本地包

至少检查：连接测试、项目分页、工作流详情、实例任务与日志、定时查询；如需写入，在测试环境检查启动、停止和调度修改。
记录 DBX / DolphinScheduler 版本。不需要把 Token 发给仓库维护者或写入 GitHub Secrets。

## 2. 发布候选版本

修改 `manifest.json` 与 `backend/main.go` 中版本号，并更新发布说明；保持插件 ID、publisher 和已有 provider/workbench ID 不变。

```sh
git add .
git commit -m "Release v0.1.1"
git push origin main
gh release create v0.1.1 --target main --title "v0.1.1" --notes "经过真实环境验证的候选版本"
```

CI 与 Release 工作流应通过。Release 页应包含：

- `io.github.crayon-hua.dolphinscheduler-console-<版本>-windows-x64.dbxp`
- 对应的 `.artifact.json`
- `release-candidates.json`

首次本地预览版可以标记为 prerelease；申请上架时发布一个经过验收的正式 Release，便于商店自动同步器找到最新版本。
版本标签必须为 `v` 加 manifest 版本，匹配源码。标签和已发布安装包保持不可变，修改代码后使用新版本。

发布工作流使用官方可复用工作流 `plugin-cli-v0.1.10`，只构建当前已验证的 Windows x64 目标。
其他平台需要先测试，再增加 `build-matrix`。无需配置 DBX Store 私钥。

若需手动重试工作流，选择与目标 Release 相同的 tag/ref，填写相同 `release_tag`。不要覆盖已发布版本的二进制；打新版本更稳妥。

## 3. 生成商店提交材料

下载 Release 的 `release-candidates.json` 到仓库根目录，然后运行：

```sh
python scripts/store_candidate.py release-candidates.json
```

生成的 `store-submission/` 包含：

- `publishers/crayon-hua.json`：首次申请发布者记录，`status` 为 `unverified`。
- `candidates/io.github.crayon-hua.dolphinscheduler-console.json`：候选安装包 HTTPS URL、SHA-256 和大小。

该脚本以 Release 真实元数据为输入，不自行编造 hash 或签名。先检查版本、链接与身份，再提交。
如果发布者已存在，则不再提交重复的 publisher 文件。

## 4. 向 DBX Store 提交 PR

1. Fork [t8y2/dbx-store](https://github.com/t8y2/dbx-store)。
2. 创建新分支，把上一步的 publisher（首次才需要）和 candidate 文件放到对应目录。
3. 在 store 仓库按其当前说明运行 `node scripts/validate.mjs`。
4. 向 `t8y2/dbx-store` 的 `main` 分支提交 PR，按模板说明功能、权限、网络与原生后端行为。

可填写以下审核信息：

> DolphinScheduler 工作台，提供 31 个项目、工作流、实例、调度和日志工具。用户自行保存连接，Token 使用 DBX Secret Store。Go 原生 sidecar 仅通过用户配置的 API endpoint 请求 DS，不启动本地监听服务、不执行 shell、不访问业务数据库。支持系统证书验证的 HTTP/HTTPS，禁止重定向以免转发 Token。修改操作需非只读连接及 confirm=true。返回日志、定义、实例数据仅用于工作台和用户授权的 AI/MCP 调用。插件没有申请额外 Host 权限。

补充你真实测试过的版本与操作。原生 sidecar 使用当前用户权限运行，不能声称其受到操作系统沙箱隔离。

不要上传 `.dbxp`、源码副本、Token 或私钥到 store Git 历史；不要手工修改 `plugins/*.json` 或 `catalog/index.json`，也不要自行设置 verified / signingKeyId。

## 5. 等待审核、签名、合并

候选 PR 最初可能显示 `open candidate(s) awaiting DBX Store signing` 的红色检查，这是预期状态。
维护者审核源码与固定 hash 的候选包，运行受保护的签名流程，生成最终商店记录，检查通过后合并。
审核与上架没有保证时限。只有维护者完成审核、签名、合并后，插件才进入官方目录。

首次上架时还可申请把本仓库注册到 store 的 `automation/plugin-sources.json` 并开启 `autoUpdate: true`。
之后只需发布带 `release-candidates.json` 的新 Release，同步器会生成候选 PR；更新仍需维护者审核签名。
自动同步 App 的凭据属于 store 管理方，本插件仓库不需要这些 Secrets。

## 官方规则

- [DBX 插件开发与发布](https://dbxio.com/en/docs/plugin-development)
- [DBX Store CONTRIBUTING](https://github.com/t8y2/dbx-store/blob/main/CONTRIBUTING.md)

以上按 2026-11-11 查询到的规则整理；提交时以 store 当前规则为准。
