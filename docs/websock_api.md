**WebSocket 接入说明**

- 连接地址：`ws://<host>:<port>/api/v1/ws/join`（示例：`ws://localhost:8080/api/v1/ws/join`）。
- 心跳：服务端每 30s 发送 `Ping`，45s 未收到 `Pong` 会关闭。大多数 WebSocket 库会自动回复 `Pong`，否则需手动回复。

**消息总封装**

- 请求（客户端 → 服务端）：
  - `request_type`: string，取值见下表。
  - `data`: 对应请求体的 JSON 对象。
- 响应（服务端 → 客户端）：
  - `response_type`: string，取值见下表。
  - `data`: 对应响应体的 JSON 对象。
  - `error_message`: string，可选；当有错误时携带。
- 必须保证字段完整，不要省略可选字段（即使为空字符串也按协议字段名发送）。

**玩家与角色模型**

- 玩家：`id`、`name`、`role`、`word`（可为空，`omitempty`，白板为空字符串，管理员/观察者通常无词）。
- 角色枚举：`Unset`（未分配，等待阶段的普通玩家）、`Admin`（首个加入）、`Normal`、`Blank`、`Spy`、`Observer`（超出 8 人或游戏已开始后加入）。

**请求类型与数据**

1. `JoinGame`

```json
{
  "request_type": "JoinGame",
  "data": {
    "room_id": "string", // 必填，房间 ID
    "joiner_name": "string" // 必填，玩家昵称
  }
}
```

- 首条消息必须是 `JoinGame` 才会加入房间并获得后续 req 通道。

响应（服务端 → 客户端）：`JoinGame` 的行为稍有区分：

- 正常加入（新玩家）时，服务端会广播一条 `JoinGame` 给房间内所有连接，`data` 包含公开的房间快照：

```json
{
  "response_type": "JoinGame",
  "data": {
    "room_id": "string",
    "stage": "Waiting|Preparing|Speaking|Voting|Judging|Finished",
    "joiner": { "id": "string", "name": "string", "role": "...", "word": "" },
    "players": [ { "id": "string", "name": "string", "role": "...", "word": "" }, ... ],
    "master_id": "string"
  }
}
```

- 断线重连（检测到同名玩家）时，服务器会先**单发（私发）**给重连者一条包含其完整信息的 `JoinGame`（`joiner.word` 与 `joiner.role` 为完整值），随后再**广播（公开）**一条 `JoinGame` 给房间内所有人，其中 `joiner` 与 `players` 列表均为公开视图（`word` 字段被清空以防泄露）。

- 公开视图的 `players` 用于前端重建玩家列表与当前阶段，不含任何玩家的秘密词（`word` 均为空）。

- `master_id` 表示房主/管理员的 player id，用于前端显示/权限控制。

- `master_id` 表示房主/管理员的 player id，用于前端显示/权限控制。

2. `SetWords`

```json
{
  "request_type": "SetWords",
  "data": {
    "set_player_id": "string", // 必填，必须是管理员 ID
    "word_list": ["normalWord", "spyWord"] // 必填，只能包含两个词：索引 0 为正常词，索引 1 为卧底词
  }
}
```

3. `StartGame`

```json
{
  "request_type": "StartGame",
  "data": {
    "start_player_id": "string" // 必填，必须是管理员 ID
  }
}
```

- 管理员必须先通过 `SetWords` 设置词语（至少两个词，索引 0 为正常词、索引 1 为卧底词），否则服务端会拒绝开始请求并返回错误。
- 至少 8 个 `Unset` 玩家才允许开始，否则服务端记录错误（不会推送成功响应）。

- 当管理员成功触发开始时，服务端会向每个参与者单播其 `assigned_role`/`assigned_word`。此外，**管理员会收到一条仅发给管理员的 `StartGame` 响应，响应的 `data` 中包含 `players` 字段，列出房间内所有玩家的完整信息（包含 `id`、`name`、`role`、`word`）以便管理员界面展示与确认**。普通参与者与观察者收到的 `StartGame` 响应不包含该 `players` 列表或该字段为空。

4. `Describe`

```json
{
  "request_type": "Describe",
  "data": {
    "req_player_id": "string", // 必填，当前轮到发言的玩家 ID
    "message": "string" // 必填，发言内容
  }
}
```

5. `Vote`

```json
{
  "request_type": "Vote",
  "data": {
    "voter_id": "string", // 必填，投票人 ID（存活且非管理员/观察者）
    "target_id": "string" // 必填，被投票人 ID（存活且非管理员/观察者）
  }
}
```

6. `Timeout`（保留，服务端内部计时用；客户端无需发送）

```json
{
  "request_type": "Timeout",
  "data": {
    "stage": "Waiting|Preparing|Speaking|Voting|Judging|Finished"
  }
}
```

。

**响应类型与数据**

1. `Error`

```json
{
  "response_type": "Error",
  "data": null,
  "error_message": "string"
}
```

2. `JoinGame`

```json
{
  "response_type": "JoinGame",
  "data": {
    "room_id": "string",
    "stage": "Waiting|Preparing|Speaking|Voting|Judging|Finished",
    "joiner": {
      "id": "string",
      "name": "string",
      "role": "Admin|Unset|Normal|Blank|Spy|Observer",
      "word": "string"
    },
    "players": [
      {
        "id": "string",
        "name": "string",
        "role": "Admin|Unset|Normal|Blank|Spy|Observer",
        "word": "string"
      }
    ],
    "master_id": "string"
  }
}
```

- 说明：服务端根据场景会发送两种 `JoinGame`：
  - **私发（仅发给重连者）**：`data.joiner.word` 与 `data.joiner.role` 为完整值，用于恢复该玩家的私有信息；`data.players` 为公开列表（`word` 字段为空）。
  - **广播（发给所有人）**：`data.joiner` 与 `data.players` 均为公开视图，所有玩家的 `word` 字段均为空以防泄露。

3. `SetWords`

```json
{
  "response_type": "SetWords",
  "data": {
    "word_list": []
  }
}
```

- 只有管理员可以设置词语，且服务器不会在广播中泄露实际词语；响应中的 `word_list` 为空数组或仅表示设置成功。管理员提供的词语仅用于服务端在开始阶段给参与者单播分配，公共广播不会包含敏感词语。

**ExitGame**

```json
{
  "response_type": "ExitGame",
  "data": {
    "left_player_id": "string",
    "left_player_name": "string"
  }
}
```

- 行为说明：服务端会在玩家退出时发送 `ExitGame` 响应给相关连接：
  - 向退出的连接单播一条 `ExitGame` 作为退出确认；
  - 同时向房间内其他连接广播一条 `ExitGame` 通知，通知中字段与单播一致（`left_player_id` / `left_player_name`）。
  - 文档中不暴露任何关于服务端内部触发退出请求的细节；客户端只需处理收到的 `ExitGame` 响应即可。

4. `StartGame`

````json
{
  "response_type": "StartGame",
  "data": {
    "assigned_role": "Normal|Blank|Spy",
    "assigned_word": "string", // Blank 为空字符串
    "players": [
      {
        "id": "string",
        "name": "string",
        "role": "Admin|Unset|Normal|Blank|Spy|Observer",
        "word": "string" // 管理员视图下可能包含真实词语；普通玩家/观察者通常不接收此字段的敏感值
      }
    ]
  }
}

- 说明：服务端对于 `StartGame` 的发送策略如下：
  - 私发给管理员：`data.players` 包含房间内所有玩家的完整信息（可含 `word` 和 `role`），用于管理员界面展示与审查。确保前端仅在管理员权限下渲染这些敏感字段。
  - 私发给普通参与者：`data.assigned_role` 与 `data.assigned_word` 为该玩家的私有信息（`players` 字段为空或不返回）。
  - 私发给观察者：通常不包含 `players`，并且 `assigned_role`/`assigned_word` 为空字符串。
- 单播给非管理员/非观察者玩家，用于展示身份。

5. `Describe`

```json
{
  "response_type": "Describe",
  "data": {
    "speaker_id": "string",
    "speaker_name": "string",
    "message": "string"
  }
}
````

- 广播当前发言文本。

6. `Vote`

```json
{
  "response_type": "Vote",
  "data": {
    "voter_id": "string",
    "voter_name": "string",
    "target_id": "string",
    "target_name": "string"
  }
}
```

- 广播投票行为。

7. `GameState`

```json
{
  "response_type": "GameState",
  "data": {
    "stage": "Waiting|Preparing|Speaking|Voting|Judging|Finished",
    "current_turn_id": "string (可选)",
    "current_turn_name": "string (可选)",
    "round": 1
  }
}
```

- 广播阶段切换、轮次信息；发言阶段会附带当前发言者。

8. `Eliminate`

```json
{
  "response_type": "Eliminate",
  "data": {
    "eliminated_id": "string",
    "eliminated_name": "string",
    "eliminated_word": "string" // 被淘汰玩家的词语
  }
}
```

- 判定阶段淘汰后广播。

9. `GameResult`

```json
{
  "response_type": "GameResult",
  "data": {
    "winner": "卧底方|平民方",
    "answer_word": "string",
    "spy_word": "string",
    "player_roles": { "player_name": "Role", "...": "..." },
    "player_words": { "player_name": "Word", "...": "..." }
  }
}
```

- 结束阶段广播，键为玩家姓名。

**阶段与超时**

- Waiting：可 `JoinGame`、`
- SetWords`、`StartGame`。首个加入者为管理员；超过 8 人或非等待阶段加入将成为 `Observer`。
- Preparing：进入后根据管理员提供的词语确定性分配角色/词语（`word_list[0]` 为正常词，`word_list[1]` 为卧底词），并单播 `StartGame` 给每位参与者（每位参与者只会收到属于自己的 `assigned_word`，白板为空字符串）；10s 后自动进入 Speaking。
- Speaking：随机发言顺序，当前发言者 20s 超时；收到 `Describe` 后切下一位；全员发言完切 Voting。
- Voting：30s 超时；每次 `Vote` 广播；所有存活玩家投完或超时进入 Judging。
- Judging：统计最高票淘汰并广播 `Eliminate`；若卧底/白板胜或全出局则进入 Finished，否则回到 Speaking，回合数 +1；10s 后自动切 Speaking。
- Finished：广播 `GameResult`；不再处理请求。

**前端使用建议**

- 首条消息务必发送 `JoinGame`，并保存返回的 `id`、`role`。
- 只有管理员发送 `SetWords`、`StartGame`；其他请求按阶段发送，否则会被拒绝（仅日志，不回包）。
- 监听 `GameState`/`Describe`/`Vote`/`Eliminate`/`GameResult` 做 UI 更新。
- 保留并使用所有字段名，空值也发送空字符串/空数组，避免字段缺失。
