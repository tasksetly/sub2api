# Repository working rules

- 开始任何编程任务前，先阅读 `CLAUDE_DEV.md` 了解项目结构和可复用的代码。
- 写代码时优先复用项目内部已有的代码（组件、函数、工具包、handler/service/repository），不要重复造轮子。搜索 `backend/internal/` 和 `frontend/src/` 找现成实现。
- 不编译项目。写完或改完代码后，不要运行编译、构建或测试命令，除非用户明确要求。优先用 `git diff --check` 和静态检查，并说明跳过了哪些检查。
- 功能写完后提交本地 Git commit，不要 push 到远程，除非用户明确要求。
- 只提交本次任务改动的文件，不要包含无关的、早已存在的、未跟踪的或用户拥有的改动。
- 功能完成后，在 `CLAUDE_DEV.md` 的"功能清单"章节加一行记录新功能（日期、功能、说明、主要文件、Commit）。
