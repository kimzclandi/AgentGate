# 面试演示（约 6 分钟）

## 自动验收

`make demo` 构建二进制，在操作系统临时目录启动独立实例。脚本会终止自己启动的进程并清理自己的临时目录，不访问正常 data。成功检查全部写入 JSON，失败立即报错。

## 手工控制台

1. `make run`；另开终端 `AGENTGATE_DEV=1 ./bin/agentgate -token alice`。打开 localhost 控制台粘贴令牌。
2. 输入 `read-doc doc-1`：看到真实文档、成功运行和带 request/run ID 的审计。
3. 输入 `read-doc doc-2`：统一 403。说明 tenant 来自 DB 身份，用户输入不能覆盖。
4. 输入 `update-ticket ticket-1 Resolved`：先显示 pending，无业务写入。核对参数和摘要，点击确认。
5. 再输入 `read-ticket ticket-1`：看到 Resolved，证明写入完成；不是只看成功 toast。
6. 用 API 再确认同一个 action_id/digest：409 approval_replayed。更换 digest：未消费审批返回 mismatch。
7. 创建待审批更新，点击撤销当前身份；旧令牌后续返回 401。通过离线恢复命令恢复用户后，旧运行也无法执行。
8. 超时与取消演示用 `scripts/demo.py` 的固定用例，展示 1 秒运行过期后拒绝。重启前待审批动作保留，但所属运行 interrupted 后执行失败。

## 面试时说明

这是双方合作、AI 辅助开发的工程样例，必须能够解释实际代码；不要宣称从零编写 JWT/OIDC/SQLite。两个资源适配器是本地数据库应用，不是已接入真实企业系统。固定任务通过率是受限回归集的检查结果，不是通用 Agent 成功率。没有任意代码沙箱或真实付费模型结果。
