# Tasksetly 推广执行记录

> 用途：每次执行前先读取本文件；只执行状态为 `TODO` 或 `IN_PROGRESS` 的任务。完成后必须记录日期、改动文件、验证命令和结果，避免重复执行。
>
> 提交规则：每个小需求独立完成并验证后，立即更新本记录、单独提交并推送当前远程分支，再开始下一项。

## 状态说明

- `TODO`：尚未开始
- `IN_PROGRESS`：正在执行
- `DONE`：已完成且验证通过，不重复执行
- `BLOCKED`：被外部条件阻塞

## 任务清单

| ID | 状态 | 任务 | 完成日期 | 证据/备注 |
|---|---|---|---|---|
| MKT-001 | DONE | 审查线上站点、源码与产品定位 | 2026-07-28 | 已核验首页公开配置、HomeView、README 和现有 SEO 响应 |
| MKT-002 | DONE | 制定 30 天渠道推广计划和内容日历 | 2026-07-28 | `/home/debian/ai/sub2api-deploy/MARKETING_PLAN_30D.md` |
| MKT-003 | DONE | 首页 CTA 对未登录用户直达注册页 | 2026-07-28 | `landing.spec.ts` 2 个 CTA 测试通过；HomeView 使用 `/register` |
| MKT-004 | DONE | 中文首页价值主张与文档入口 | 2026-07-28 | 通用英文副标题在中文页替换为本地化文案；保留管理员自定义值 |
| MKT-005 | TODO | robots.txt、sitemap.xml、meta/OG SEO |  |  |
| MKT-006 | TODO | 首次触达 UTM 保存及注册归因 |  |  |
| MKT-007 | DONE | 快速开始文档 | 2026-07-28 | `frontend/public/docs/quickstart.html`；生产构建确认复制到 dist |
| MKT-008 | TODO | 前端测试、类型检查和生产构建 |  | 本轮局部测试与构建已通过；全量基线另有 2 个 rollback 测试失败 |
| MKT-009 | TODO | Docker 构建、推送、Compose 部署及线上验证 |  |  |
| MKT-010 | TODO | 发布 D01 项目介绍内容 |  | 需先确定具体发布账号/渠道或现有登录态 |
| MKT-011 | TODO | 发布 D02 五分钟快速开始教程 |  | 需 MKT-007 完成后发布 |

## 执行日志

### 2026-07-28

- 创建本执行记录。
- 已确认用户拥有商业运营授权；后续不再把项目商业授权作为推广阻塞项。
- 完成 MKT-003：未登录首页 CTA 由 `/login` 改为 `/register`。
- 完成 MKT-004：中文页替换通用英文副标题，并为未配置文档地址的站点提供本地快速开始入口。
- 完成 MKT-007：新增 `frontend/public/docs/quickstart.html`。
- 验证：`pnpm exec vitest run src/utils/__tests__/landing.spec.ts`，6/6 通过。
- 验证：`pnpm run build`，Vue 类型检查和 Vite 生产构建通过，快速开始页面已复制到构建产物。
- 基线说明：首次误触发全量测试时发现既有 `admin.system.rollback.spec.ts` 2 个失败，与本次改动无关。
- 下一项：MKT-005（SEO 基础文件和元信息）。
