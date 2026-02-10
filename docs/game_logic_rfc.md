# 誰是臥底游戏逻辑设计方案 (RFC)

## 1. 概述

本方案旨在基于现有的 `internal/service/game` 框架，实现一个简单的“谁是卧底”游戏逻辑。
游戏规则参考 `docs/gameplay.md`：

- 配置：标准8人局（1卧底、1白板、6平民）。人数不足时，不允许开始游戏。
- 胜利条件：
  - 卧底/白板方胜利：存活人数 < 4 且 卧底或白板尚在场。
  - 平民方胜利：卧底和白板均被淘汰。
- 流程：发言 -> 投票 -> 判决 -> 循环（最多4轮）。

## 2. 状态机设计

利用 `StageHandler` 接口，我们将游戏分为以下几个状态实现：

### 2.1 状态列表

除了现有的 `STATUS_WAITING`，我们将实现以下状态：

1.  **Preparing (准备阶段)**: 分配身份，分配词语，初始化游戏轮次信息。
2.  **Speaking (发言阶段)**: 玩家按顺序描述词语。
3.  **Voting (投票阶段)**: 玩家投票处决某人。
4.  **Judging (判定阶段)**: 计算票数，淘汰玩家，检查胜利条件。
5.  **Finished (结束阶段)**: 展示游戏结果。

### 2.2 详细逻辑

#### A. Waiting (等待阶段 - 已有基础)

- **动作**: 玩家 `JoinGame`，管理员 `SetWords`，管理员 `StartGame`。
- **转换**: 收到 `StartGame` 请求后，检查人数（至少 8 个正常玩家），转换到 `Preparing`。

#### B. Preparing (准备阶段)

- **入口逻辑 (`OnEnter`)**:
  1.  **角色分配**:
      - 随机选取1人为 `Spy` (卧底)。
      - 随机选取1人为 `Blank` (白板)。
      - 其余为 `Normal` (平民)。
  2.  **词语分配**:
      - 从 `WordList` 中选取一对词 (如 "汤圆" vs "饺子")。
      - `Normal` 获得词A，`Spy` 获得词B，`Blank` 获得空字符串。
  3.  **初始化轮次**: 设 `Round = 1`。
  4.  **初始化存活列表**: `AlivePlayers` 包含所有玩家ID。
  5.  **广播游戏开始**: 通知所有玩家其身份和词语 (`StartGameResponse` 已包含此意图)。
- **转换**: 10 秒后，自动转换到 `Speaking`。

#### C. Speaking (发言阶段)

- **数据**:
  - `SpeakingOrder`: 当前存活玩家ID列表 (乱序)。
  - `CurrentSpeakerIndex`: 当前发言者索引。
- **入口逻辑**:
  - 广播 `StageChange` (进入发言阶段)。
  - 广播 `TurnChange` (通知第一个玩家发言)。
- **处理 (`OnHandle`)**:
  - 接受 `DescribeRequest` (描述请求)。
  - 验证是否轮到该玩家。
  - 广播 `DescribeResponse` (包含 `SpeakerID`, `Message`)。
  - `CurrentSpeakerIndex++`。
  - 若所有人都已发言 -> 转换到 `Voting`。
  - 否则 -> 广播 `TurnChange` (通知下一位)。

#### D. Voting (投票阶段)

- **数据**:
  - `Votes`: map[string]string (投票者ID -> 被投者ID)。
- **入口逻辑**:
  - 广播 `StageChange` (进入投票阶段)。
  - 清空 `Votes`。
- **处理 (`OnHandle`)**:
  - 接受 `VoteRequest` (投票请求)。
  - 记录投票。
  - 检查是否所有存活玩家都已投票。
  - 若是 -> 转换到 `Judging`。

#### E. Judging (判定阶段)

- **入口逻辑**:
  1.  **计票**: 统计得票最多者。
      - _简陋策略_: 若平票，按顺序淘汰第一个最高票者（默认淘汰第一个最高票者以加速游戏）。
  2.  **淘汰**: 将该玩家从 `AlivePlayers` 移除，标记状态为 `Observer`。即，被淘汰的玩家，应该变成旁观者。
  3.  **广播结果**: 通知谁被淘汰 (公布其 word)。
  4.  **胜利检查**:
      - **BadWin**: `len(AlivePlayers) < 4` AND (`Spy` or `Blank` is Alive)。 -> 卧底方胜。
      - **GoodWin**: `Spy` AND `Blank` exist in `EliminatedPlayers` (both out)。 -> 平民胜。
      - **Continue**: 否则 -> `Round++`。
  5.  **转换**:
      - 胜负已分 -> 转换到 `Finished`。
      - 未分胜负 -> 转换到 `Speaking` (保持剩余玩家顺序)。

#### F. Finished (结束阶段)

- **入口逻辑**:
  - 广播 `GameResultResponse` (胜利方，所有玩家真实身份，卧底词/平民词)。
  - 直接结束，清理资源防止内存泄漏。

## 3. 数据结构修改建议

### GameContext 扩展

```go
type GameContext struct {
    // ... 原有字段

    // 游戏进程数据
    Round             int
    // 存活玩家通过计算 spy + blank + normal 的数量计算
    // Role 直接在 Player 结构体中有

    // 发言阶段
    SpeakingOrder     []string // 本轮发言顺序
    CurrentSpeakerIdx int

    // 投票阶段
    Votes             map[string]string // VoterID -> TargetID

    // 词汇信息
    AnswerWord        string
    UndercoverWord           string
}
```

### 新增 Action 定义 (action.go)

```go
// 描述
type DescribeRequest struct {
    ReqPlayerID string `json:"req_player_id"`
    Content     string `json:"content"`
}
type DescribeResponse struct {
    SpeakerID string `json:"speaker_id"`
    Content   string `json:"content"`
}

// 投票
type VoteRequest struct {
    VoterID  string `json:"voter_id"`
    TargetID string `json:"target_id"`
}

// 投票结果不直接显示，直到 judging 阶段再展示

// 阶段变更/轮次变更通知
type GameStateNotification struct {
    Stage       string `json:"stage"`
    CurrentTurn string `json:"current_turn,omitempty"` // 当前轮到谁
    Round       int    `json:"round"`
}
```

## 4. 接口实现计划

1. 在 `logic.go` 或新文件 `logic_stages.go` 中实现 `prepStageHandler`, `speakStageHandler`, `voteStageHandler`, `judgeStageHandler`, `finishStageHandler`。
2. 每个 Handler 需实现 `StageHandler` 接口。
3. 修改 `waitStageHandler` 的 `StartGame` 处理逻辑，使其能够初始化必要数据并触发状态切换。
4. 最后实现 GameMachine，负责管理 GameContext 和各个 StageHandler。

## 5. 简易化取舍 (Trade-offs)

- **平票处理**: 不进行PK发言，直接淘汰第一个最高票，简单粗暴。
- **断线重连**: 暂不考虑复杂重连，假设都在线。
- **并发控制**: 利用 Go channel 的单线程处理模型 (Actor模式) 避免锁的复杂性 (Context 中处理逻辑是串行的)。即可以认为 GameContext 永远是在一个单线程下运行。
- **词库**: 暂时硬编码或简单的列表，不接外部数据库。
