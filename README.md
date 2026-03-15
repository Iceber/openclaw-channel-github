# OpenClaw GitHub Channel 方案设计与实现文档

本文深入探讨将 GitHub 仓库实现为 OpenClaw Channel 的可行性，重点分析以 **Issue** 与 **Pull Request** 作为消息载体的设计方案，并给出一份可直接落地的实现文档。

目标仓库参考：

- OpenClaw 主仓库：<https://github.com/openclaw/openclaw>
- OpenClaw Channels 文档：<https://docs.openclaw.ai/channels>

---

## 1. 结论先行

### 1.1 方案是否可行

**可行，但定位应当是“异步协作型 Channel”，而不是“即时聊天型 Channel”。**

GitHub 天然不适合替代 Telegram、Discord、Slack 这类实时消息渠道，但它非常适合以下 OpenClaw 场景：

- 面向仓库的异步任务协作
- 以 Issue 为单位的问题处理和跟踪
- 以 PR 为单位的代码审查、变更讨论和自动化辅助
- 将 AI 助手作为仓库中的“协作参与者”而非“聊天机器人”

### 1.2 最适合的目标定位

建议将 GitHub Channel 定义为：

> 一个以 **仓库为边界**、以 **Issue / PR 线程为会话容器**、以 **评论 / Review / Reaction** 为消息交互形式的异步协作 Channel。

### 1.3 核心判断

这个方案最大的价值不在“聊天体验”，而在：

- 把 OpenClaw 接入开发者已经在使用的工作流
- 让 AI 直接参与需求澄清、问题诊断、代码审查和发布协作
- 利用 GitHub 原生权限模型、审计能力、Webhook 事件和 API 生态

它最大的限制在于：

- 不具备真正的实时对话体验
- 线程模型比即时通信平台更复杂
- GitHub API 限流和权限边界需要认真设计
- PR Review、普通评论、行级评论之间存在多种消息形态

---

## 2. OpenClaw Channel 模型理解

基于 OpenClaw 已公开的 Channels 与 CLI 文档，可以归纳出一个 GitHub Channel 需要遵循的核心模型。

### 2.1 Channel 在 OpenClaw 中是什么

OpenClaw 的 Channel 可以理解为：

- 一个外部沟通平台接入器
- 一个把外部消息规范化后送入 Gateway 的消息源
- 一个把 Agent 输出重新投递回外部平台的消息目的地

现有 Channel 主要覆盖：

- Telegram
- Discord
- Slack
- WhatsApp
- Signal
- iMessage / BlueBubbles
- Matrix
- Mattermost
- Google Chat
- 其他插件型渠道

这些渠道虽然协议不同，但 OpenClaw 的共性抽象大致一致：

- **入站事件**：外部平台消息进入 Gateway
- **会话键**：将消息映射为一个可持续的 session key
- **路由绑定**：决定该消息应由哪个 agent 处理
- **安全策略**：DM / Group / Allowlist / Mention gating
- **出站投递**：将 agent 输出发送回原始对话上下文

### 2.2 现有 Channel 的关键约束

从 OpenClaw 的公开文档中，GitHub Channel 设计至少要兼容以下约束：

1. **Gateway 是会话与状态的唯一真相源**
2. **Session key 设计必须稳定且可复现**
3. **多 Channel、多账号与 Agent 绑定是常见场景**
4. **群组与私聊通常有不同安全策略**
5. **消息平台能力不一致，能力差异应通过 capability / degrade 处理**

### 2.3 GitHub 与现有 Channel 的本质区别

GitHub 不是“聊天工具”，更像一个：

- 结构化协作平台
- 带审计能力的任务系统
- 带事件流的代码协作系统

因此 GitHub Channel 不应照搬 IM 模型，而应进行以下映射：

- 仓库 ≈ Channel account / workspace
- Issue / PR ≈ 会话容器
- Comment / Review / Review Comment ≈ 消息
- Mention / Assignment / Label / State Change ≈ 上下文事件
- Bot comment / review summary ≈ OpenClaw 出站消息

---

## 3. GitHub Channel 的目标与非目标

### 3.1 目标

实现一个能够让 OpenClaw 在 GitHub 仓库中完成以下能力的 Channel：

- 监听指定仓库的 Issue / PR 事件
- 将 Issue / PR 及评论转成 OpenClaw 入站消息
- 把 Agent 的输出回写为 GitHub 评论、PR Review 或 Reaction
- 用仓库、Issue/PR 编号、评论上下文建立稳定会话
- 支持仓库级安全控制、Allowlist、触发条件和限流

### 3.2 非目标

首期不建议把以下内容纳入必须范围：

- 把 GitHub 变成真正的私聊渠道
- 完整支持所有 GitHub 事件
- 首版就支持多安装、多组织、多账号复杂路由
- 首版就支持完整的代码行级 review thread 恢复和编辑同步
- 首版就支持所有媒体能力

---

## 4. 可行性分析

### 4.1 为什么可行

#### 4.1.1 GitHub 已具备成熟的事件输入机制

GitHub 提供了稳定的 Webhook 事件流，适合做 Channel 入站层：

- `issues`
- `issue_comment`
- `pull_request`
- `pull_request_review`
- `pull_request_review_comment`
- `discussion` / `discussion_comment`（可作为未来扩展）

这些事件天然适合映射为 OpenClaw 的 inbound message / context update。

#### 4.1.2 GitHub 已具备完善的消息输出接口

GitHub REST / GraphQL API 支持：

- 创建 Issue 评论
- 创建 PR 评论
- 创建 PR Review
- 添加 Reaction
- 获取仓库、Issue、PR、Review、Comment 元信息

因此 OpenClaw 可以像在聊天平台中“回复消息”一样，在 GitHub 中“回复评论或发起 review”。

#### 4.1.3 GitHub 非常适合异步协作 Agent

GitHub 的天然工作流与 OpenClaw 的能力高度匹配：

- Issue triage
- bug 分析
- PR review 建议
- 需求澄清
- 自动问答
- 自动生成变更说明
- CI / 构建 / 发布上下文解释

### 4.2 为什么不能简单照搬 IM Channel

GitHub 与聊天平台最大的不同在于：

#### 4.2.1 会话不是“用户对话”，而是“工单线程”

即时消息渠道的会话中心通常是：

- 用户
- 群
- 频道
- 线程

GitHub 的会话中心则是：

- 某个仓库中的一个 Issue
- 某个仓库中的一个 PR
- 进一步细化时，是某个 review thread

#### 4.2.2 消息有强结构语义

GitHub 中的“消息”并不只有一种：

- Issue body
- Issue comment
- PR body
- PR comment
- Review summary
- Inline review comment
- State change event

如果全部当作普通文本处理，会丢失重要语义。

#### 4.2.3 输出不只是回复文本

OpenClaw 在 GitHub 中的输出可能包括：

- 发表评论
- 发起 review
- 提交 approve / comment / request changes
- 添加 reaction
- 更新标签或状态（未来扩展）

这意味着 GitHub Channel 更像“协作动作接口”而不只是“消息通道”。

### 4.3 综合评估

| 维度 | 评估 |
| --- | --- |
| 技术可行性 | 高 |
| 与 OpenClaw 架构契合度 | 中高 |
| 作为实时聊天替代品 | 低 |
| 作为异步协作 Channel | 很高 |
| 首版实现复杂度 | 中 |
| 长期扩展价值 | 很高 |

---

## 5. Channel 抽象映射设计

本节给出 GitHub 平台对象到 OpenClaw Channel 抽象的映射方案。

### 5.1 账户与租户模型

建议采用 **GitHub App** 作为首选接入方式，而非个人 Token。

原因：

- 安装粒度可以控制到组织 / 仓库
- 权限模型更清晰
- 更符合生产化部署
- 更容易做 webhook 验签和事件归属

映射关系建议如下：

| GitHub 概念 | OpenClaw 概念 |
| --- | --- |
| GitHub App Installation | channel account |
| owner/repo | conversation namespace |
| issue / pull request | session container |
| comment / review | message |
| bot account | channel sender identity |

### 5.2 会话模型

建议把 GitHub Channel 视为“群组 / 线程型渠道”，而不是 DM 渠道。

推荐 session key 设计：

```text
agent:<agentId>:github:<owner>/<repo>:issue:<number>
agent:<agentId>:github:<owner>/<repo>:pr:<number>
```

如果后续要支持更细的 review thread，可扩展为：

```text
agent:<agentId>:github:<owner>/<repo>:pr:<number>:review-thread:<threadId>
```

设计原则：

- **稳定**：同一 Issue / PR 必须始终映射到同一 session
- **可恢复**：重启后可通过 webhook payload 重新定位
- **可分级**：先以 Issue / PR 为粒度，未来再细化到子线程

### 5.3 消息映射

#### 5.3.1 入站消息

建议的基础映射如下：

| GitHub 事件 | 是否入站消息 | 备注 |
| --- | --- | --- |
| issue opened | 是 | issue body 作为首条上下文消息 |
| issue edited | 可选 | 作为上下文更新事件，不一定触发 agent |
| issue_comment created | 是 | 最核心消息来源 |
| pull_request opened | 是 | PR body 作为首条上下文消息 |
| pull_request_review submitted | 是 | 作为 review 级消息 |
| pull_request_review_comment created | 是 | 行级 review 评论 |
| labeled / unlabeled | 可选 | 更适合作为 context event |
| assigned / unassigned | 可选 | 更适合作为 metadata update |
| closed / reopened / merged | 可选 | 作为状态事件 |

#### 5.3.2 出站消息

建议首版支持以下出站动作：

| OpenClaw 输出类型 | GitHub 落点 |
| --- | --- |
| 普通文本回复 | issue / PR comment |
| 总结性审查意见 | PR review comment 或 review summary |
| 轻量反馈 | reaction |

首版不建议直接让 agent 自由执行：

- merge PR
- close issue
- 修改分支保护
- 直接 push 代码

这些应该由更严格的工具权限和人工审批控制。

### 5.4 用户身份映射

GitHub 中应保留原始用户身份信息：

- `user.login`
- `user.id`
- `author_association`
- 仓库角色信息（可选）

建议在规范化消息上下文中携带：

```json
{
  "provider": "github",
  "senderId": "github:123456",
  "senderLogin": "alice",
  "senderDisplayName": "alice",
  "repository": "openclaw/openclaw",
  "threadType": "issue",
  "threadNumber": 123
}
```

这可用于：

- allowlist / trigger 判断
- 审计日志
- identity link 扩展

---

## 6. 推荐的产品交互模型

### 6.1 触发模型

GitHub 不适合“每条消息都自动回复”，建议采用显式触发优先的策略。

推荐支持以下触发方式：

1. **@mention 触发**
   - 用户在评论中 `@openclaw-bot`
   - 最适合 PR / Issue 中的定向提问

2. **指令前缀触发**
   - 例如 `/openclaw summarize`
   - 适合固定工作流

3. **标签触发**
   - 例如加上 `ai-review`、`needs-triage`
   - 适合自动化流水线

4. **自动触发**
   - issue 打开后自动欢迎
   - PR 打开后自动进行初步分析
   - 应作为可配置能力，默认谨慎开启

### 6.2 回复模型

建议按上下文决定输出形式：

- Issue 场景：默认使用普通 comment
- PR 全局审查：默认使用 review summary 或普通 comment
- PR 行级定位：未来可扩展为 inline review comment

### 6.3 安全触发建议

首版默认策略建议：

- 仅在被 mention 或命中特定命令时才触发
- 仅允许指定仓库
- 仅允许 issue / PR comment，暂不自动响应所有 metadata event
- 仅允许 bot 账号拥有最小 GitHub 权限

---

## 7. 配置设计

以下配置风格尽量贴近 OpenClaw 现有 channel 配置习惯。

### 7.1 最小配置

```json5
{
  channels: {
    github: {
      enabled: true,
      mode: "app",
      appId: "123456",
      installationId: "78901234",
      privateKey: "secretref:github-app-private-key",
      webhookSecret: "secretref:github-webhook-secret",
      repositories: ["openclaw/openclaw"],
      issuePolicy: "allowlist",
      prPolicy: "allowlist",
      trigger: {
        requireMention: true,
        commands: ["/openclaw"],
        labels: ["ai-review", "ai-help"],
      },
    },
  },
}
```

### 7.2 多仓库 / 多账号扩展配置

```json5
{
  channels: {
    github: {
      enabled: true,
      accounts: {
        default: {
          mode: "app",
          appId: "123456",
          installationId: "111",
          privateKey: "secretref:github-default-private-key",
          webhookSecret: "secretref:github-default-webhook-secret",
          repositories: ["org-a/repo-1", "org-a/repo-2"],
        },
        enterprise: {
          mode: "app",
          appId: "654321",
          installationId: "222",
          privateKey: "secretref:github-enterprise-private-key",
          webhookSecret: "secretref:github-enterprise-webhook-secret",
          repositories: ["org-b/repo-x"],
        },
      },
      trigger: {
        requireMention: true,
      },
    },
  },
}
```

### 7.3 建议新增配置项

| 配置项 | 说明 |
| --- | --- |
| `mode` | `app` 或 `token`，推荐 `app` |
| `appId` | GitHub App ID |
| `installationId` | 安装实例 ID |
| `privateKey` | GitHub App 私钥 |
| `webhookSecret` | Webhook 验签密钥 |
| `repositories` | 允许接入的仓库 allowlist |
| `issuePolicy` | issue 触发策略 |
| `prPolicy` | PR 触发策略 |
| `trigger.requireMention` | 是否要求 `@bot` 才触发 |
| `trigger.commands` | 支持的命令前缀 |
| `trigger.labels` | 可触发 AI 的标签 |
| `outbound.mode` | `comment` / `review` / `auto` |
| `rateLimit.maxEventsPerMinute` | 入站限流 |
| `ignoreBots` | 是否忽略 bot / app 评论 |

---

## 8. 架构设计

### 8.1 总体架构

建议采用如下逻辑链路：

```text
GitHub Webhook
  -> GitHub Channel Receiver
  -> Signature Verification
  -> Event Normalizer
  -> Session Resolver
  -> OpenClaw Gateway Inbound
  -> Agent Execution
  -> GitHub Outbound Adapter
  -> Issue/PR Comment or Review
```

### 8.2 模块划分

建议按职责拆分为以下模块。

#### 8.2.1 `auth`

负责：

- GitHub App JWT 生成
- Installation token 获取与缓存
- Webhook secret 验签

#### 8.2.2 `events`

负责：

- 解析 webhook headers
- 识别事件类型与 action
- 把 GitHub payload 规范化为统一内部事件

#### 8.2.3 `normalizer`

负责把 GitHub 事件转成 OpenClaw 可消费的 inbound message / context event：

- issue body -> initial thread message
- comment -> user message
- review -> review message
- metadata change -> context update

#### 8.2.4 `routing`

负责：

- 解析 account / repository / installation
- 计算 session key
- 查找 agent bindings
- 决定是否允许触发

#### 8.2.5 `outbound`

负责：

- 发送 issue comment
- 发送 PR comment
- 创建 review summary
- 添加 reaction

#### 8.2.6 `state`

负责：

- 幂等事件处理
- 去重
- 记录 comment 与 outbound message 的映射
- 重试状态

---

## 9. 事件与消息设计

### 9.1 推荐首版支持的入站事件

#### 必须支持

- `issue_comment.created`
- `issues.opened`
- `pull_request.opened`
- `pull_request_review.submitted`
- `pull_request_review_comment.created`

#### 建议支持

- `issues.edited`
- `issue_comment.edited`
- `pull_request.edited`
- `issues.closed`
- `pull_request.closed`
- `pull_request.synchronize`

#### 后续支持

- `discussion`
- `discussion_comment`
- `check_run`
- `workflow_run`

### 9.2 规范化消息格式建议

```json
{
  "provider": "github",
  "accountId": "default",
  "repository": "openclaw/openclaw",
  "thread": {
    "type": "pull_request",
    "number": 123,
    "title": "feat: add github channel",
    "url": "https://github.com/openclaw/openclaw/pull/123"
  },
  "message": {
    "type": "comment",
    "id": "comment-456",
    "text": "@openclaw 请帮我总结这次变更的风险",
    "createdAt": "2026-03-15T00:00:00Z"
  },
  "sender": {
    "id": "github:10001",
    "login": "alice",
    "association": "MEMBER"
  },
  "trigger": {
    "kind": "mention"
  }
}
```

### 9.3 元数据处理原则

以下数据不一定当成“用户消息”，但应保留到上下文中：

- label 变化
- assignee 变化
- review state
- merge state
- CI 状态链接

原因是这些信息经常影响 agent 的判断，但不一定值得触发一次完整回答。

---

## 10. 安全设计

### 10.1 认证方式选择

首选 **GitHub App**，不推荐默认使用 PAT。

原因：

- 权限可最小化
- 审计边界清晰
- 多仓库管理更自然
- Webhook 事件归属更清晰

### 10.2 Webhook 安全

必须实现：

- `X-Hub-Signature-256` 校验
- 请求时间窗口限制
- 幂等 delivery id 去重
- 非 allowlist 仓库直接拒绝

### 10.3 权限最小化建议

首版推荐最小权限：

- Issues: Read / Write
- Pull requests: Read / Write
- Metadata: Read
- Contents: Read（仅当需要补充读取文件或 diff）

避免默认开启：

- Administration
- Actions write
- Contents write

### 10.4 Bot 循环防护

必须避免 bot 自己触发自己：

- 忽略 GitHub App 自身发出的 comment
- 忽略已标记为 OpenClaw outbound 的消息
- 对自动输出添加隐藏 marker 或 metadata 以便回环过滤

### 10.5 仓库级隔离

建议每个仓库至少在以下层面隔离：

- allowlist 配置
- rate limit
- session namespace
- 审计日志

---

## 11. 可靠性设计

### 11.1 幂等与去重

GitHub Webhook 可能重试投递，因此必须基于以下字段去重：

- delivery id
- event type
- payload object id
- action

建议保存短期幂等记录。

### 11.2 限流与退避

GitHub API 有明确限流，必须设计：

- outbound token bucket
- 429 / secondary rate limit backoff
- review/comment 批量合并
- 长回答分片策略

### 11.3 长文本处理

GitHub 评论不是无限长度，建议：

- 长输出进行摘要优先
- 必要时拆分多条 comment
- PR review 场景优先输出结构化摘要

### 11.4 失败恢复

当 outbound 失败时：

- 保留 Gateway 内执行结果
- 标记为待重试
- 最多重试 N 次
- 失败后写入错误日志并可选通知维护者

---

## 12. 详细实现方案

下面给出一个建议的分阶段实现路径。

### 12.1 Phase 1：最小可用版本

目标：

- 支持单 GitHub App installation
- 支持单仓库或少量 allowlist 仓库
- 支持 Issue / PR comment 触发
- 支持 bot comment 回写

范围：

1. webhook receiver
2. signature verification
3. installation token client
4. comment event normalizer
5. session resolver
6. outbound issue / PR comment sender

首版触发策略建议：

- 必须 `@openclaw-bot` 或命中 `/openclaw`
- 仅处理 `issue_comment.created`
- PR comment 统一先落为普通 comment，而不是 inline review

### 12.2 Phase 2：PR 审查增强

目标：

- 支持 `pull_request.opened`
- 支持 `pull_request_review.submitted`
- 支持 review summary 输出

新增能力：

- PR 打开时自动摘要
- 基于 PR 元数据生成审查建议
- 支持 approve / comment / request changes 的受控映射

### 12.3 Phase 3：上下文增强

目标：

- 把 label、assignee、merge state、CI 状态作为上下文输入
- 支持 edited / deleted 事件的上下文同步
- 支持 review thread 精细化

### 12.4 Phase 4：高级工作流

目标：

- 多 installation / 多账号支持
- discussion 支持
- 与 CI / Actions 联动
- 与 OpenClaw routing / capability / channel CLI 深度集成

---

## 13. OpenClaw 内部落地建议

虽然本仓库目前只承载方案文档，但若未来并入 OpenClaw 主项目，建议采用以下集成方式。

### 13.1 Channel 类型命名

建议使用：

```text
github
```

### 13.2 CLI 集成方向

未来可接入：

- `openclaw channels add --channel github`
- `openclaw channels list`
- `openclaw channels status --channel github`
- `openclaw channels capabilities --channel github`
- `openclaw channels logs --channel github`

### 13.3 capability 建议

GitHub Channel 能力矩阵建议如下：

| 能力 | 支持情况 |
| --- | --- |
| text inbound | 支持 |
| text outbound | 支持 |
| reaction | 支持 |
| edit awareness | 建议支持 |
| delete awareness | 可选 |
| attachment send | 有限支持 |
| thread reply | 支持 |
| rich review output | 逐步支持 |
| realtime typing / presence | 不支持 |

---

## 14. 风险与应对

### 14.1 风险：线程语义复杂

PR review、普通评论、行级评论不是一套结构。

应对：

- 首版只统一支持 issue/PR 普通 comment
- 把 inline review 留到第二阶段

### 14.2 风险：API 限流

大型仓库和高活跃 PR 容易触发限流。

应对：

- 触发条件收紧
- 评论合并输出
- 队列化 outbound
- 支持退避重试

### 14.3 风险：Bot 自激活循环

Bot 可能评论后又被 webhook 当作新消息处理。

应对：

- 忽略 bot / app actor
- 添加 outbound marker
- 使用 delivery 去重和 actor 过滤双保险

### 14.4 风险：安全边界过大

如果给了过多 repo 权限，AI 工具误操作影响会很大。

应对：

- 最小权限
- 默认只读上下文 + 评论写
- 高风险动作走显式工具授权

### 14.5 风险：用户期望错位

如果把 GitHub Channel 宣传成“聊天机器人”，体验会不匹配。

应对：

- 文档中明确其为异步协作通道
- 强调适合 issue triage、PR review、需求澄清

---

## 15. 测试与验收建议

本仓库当前没有现成测试基础设施，因此更适合先定义验收面。

### 15.1 单元测试建议

覆盖以下内容：

- webhook signature verification
- session key generation
- trigger matching
- comment payload normalization
- bot loop prevention
- outbound payload builder

### 15.2 集成测试建议

使用 webhook fixture 验证：

- issue opened
- issue_comment created
- pull_request opened
- pull_request_review submitted
- pull_request_review_comment created

### 15.3 端到端验收建议

使用一个测试仓库：

1. 创建 issue
2. 评论 `@openclaw-bot summarize`
3. 验证 OpenClaw 是否生成 comment
4. 创建 PR
5. 请求 review
6. 验证 review summary 是否正确回写

### 15.4 关键验收标准

- 非 allowlist 仓库不会被处理
- bot 自身评论不会触发回环
- 同一 Issue / PR 始终命中同一 session
- 触发条件不满足时不自动回复
- GitHub API 失败时不会丢失内部执行痕迹

---

## 16. 推荐实施顺序

为了降低复杂度，建议实施顺序如下：

1. **先做 Issue/PR 普通评论 Channel**
2. **再做 PR review summary**
3. **再做 metadata/context 增强**
4. **最后做 inline review 与高级自动化**

即：

### 第一阶段最小闭环

- GitHub App 接入
- Webhook 验签
- 评论入站
- comment 出站
- 会话路由

### 第二阶段增强闭环

- PR opened 自动分析
- review summary
- trigger label

### 第三阶段生产能力

- 多账号
- 限流
- 幂等
- 观察性
- 完整审计

---

## 17. 最终建议

如果问题是：

> “能不能把 GitHub 仓库实现成 OpenClaw 的一个 Channel，并把 Issue/PR 当作消息 Channel 使用？”

最终答案是：

**能，而且很值得做。**

但正确的产品定义应当是：

- 它不是一个“即时聊天渠道”
- 它是一个“开发协作与任务线程渠道”
- 它应围绕 Issue、PR、Review 的异步工作流进行设计

### 推荐的首版策略

- 接入方式：GitHub App
- 会话容器：Issue / PR
- 触发方式：mention + command 为主
- 出站形式：普通 comment 为主，review summary 为辅
- 安全边界：仓库 allowlist + 最小权限 + bot 回环过滤

### 推荐的实现原则

- 先保证消息模型稳定
- 再增加高级 review 语义
- 先把 GitHub 当作“协作线程系统”
- 不把它当作“聊天软件替代品”

---

## 18. 参考依据

本文设计依据主要来自以下公开资料与架构约束：

- OpenClaw 项目：<https://github.com/openclaw/openclaw>
- OpenClaw Channels 文档：<https://docs.openclaw.ai/channels>
- OpenClaw CLI Channels 说明：`docs/cli/channels.md`
- OpenClaw Session Management：`docs/concepts/session.md`
- OpenClaw Groups / Channel 行为：`docs/channels/groups.md`
- OpenClaw Chat Channels 总览：`docs/channels/index.md`

如果后续要把该方案真正实现为可运行插件，建议在 OpenClaw 主仓库中继续补充：

- GitHub Channel 配置 schema
- webhook receiver 实现
- GitHub App 凭证管理
- channel capability 定义
- CLI 管理与状态探测命令
