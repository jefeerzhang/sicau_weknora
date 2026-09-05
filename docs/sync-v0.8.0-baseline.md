# Ticket 0 基线：sync/upstream-v0.8.0 @ v0.8.0

> 记录时间：2026-09-05  
> 分支：`sync/upstream-v0.8.0`  
> Worktree：`.worktrees/sync-upstream-v0.8.0`  
> `git describe --tags`：`v0.8.0`  
> HEAD：`1edcd54b43606d9079bb36650efe3f68707a79ea`

## 分支验收

| 项 | 结果 |
| --- | --- |
| 自 `refs/tags/v0.8.0` 创建 | 是 |
| `main` 未因 Ticket 0 改业务代码 | 是（业务树仍在主工作区 `main`） |
| 与 `upstream/main` 关系 | 故意钉死 tag，不跟漂 |

## Go 测试

| 项 | 结果 |
| --- | --- |
| 宿主机 `go` | **不可用**（PATH 中无 `go`；`go.mod` 要求 `go 1.26.0`） |
| `WeKnora-app` 容器内 `go` | **不可用**（生产镜像无工具链） |
| `make test` / `go test ./...` | **未执行**（缺工具链） |

待本机安装 Go 1.26+ 或具备带工具链的开发容器后补跑，建议命令：

```bash
cd .worktrees/sync-upstream-v0.8.0
go test ./...
# 或聚焦 Ticket A 相关：
go test ./internal/application/service/ ./internal/handler/ ./internal/router/ -count=1
```

分类：环境缺口（非上游行为回归）。

## 前端测试

环境备注：worktree 内暂用 junction 指向主仓 `frontend/node_modules`（0.7.2 线依赖），**不是** 0.8.0 的干净 `npm ci`。结果仅作参考基线。

| 项 | 结果 |
| --- | --- |
| 命令 | `npm test`（worktree `frontend/`） |
| 总计 | 580 |
| 通过 | 577 |
| 失败 | 3 |

失败用例（均在 `SandboxConfigEditorDrawer.network.test.mjs`）：

1. `stored network secret recoverability requires the loaded identity` — 断言 shared recoverability helper 必须存在  
2. `domain allow without deny-all is warned about in place` — 断言 deny-all classifier 必须存在  
3. `cube L7 rules collapse to a name bar` — 断言 `moveCubeRule` 必须存在  

分类：**待用 0.8.0 锁定依赖 `npm ci` 后复测**。若仍失败，记为上游/测试资产问题并在 Ticket G（沙箱默认关）中决定是否豁免。

与川农相关的正面信号（抽样已绿）：

- `docker sandbox stays hidden unless the deployment explicitly enables it` — 与不变量 I14 方向一致  
- deployment capability / settings access 相关 10 项抽样全绿  

## Ticket A 准备（不在本 Ticket 搬代码）

川农侧将移植的测试大致位于主仓（`main`）例如：

- `internal/application/service/tenant_invitation*`  
- `internal/application/service/tenant_member*`  
- `internal/handler/tenant_invitation*` / `tenant_member*`  
- 教学迁移 / P1 相关测试  

下一步：Ticket A — RED 移植上述测试到本分支。

## 结论

Ticket 0 **完成**：基座分支与 worktree 就位；基线已归档。Go 全量与前端干净依赖复测列为环境后续项，不阻塞开始 Ticket A 的测试移植（RED 阶段）。
