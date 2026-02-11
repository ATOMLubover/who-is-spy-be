````markdown
**HTTP 接口说明**

- 基本前缀：`http://<host>:<port>/api/v1`（示例：`http://localhost:8080/api/v1`）。

**可用接口**

1. 创建房间

- 路径：`POST /rooms/create`
- 描述：创建一个新的房间并返回房间 ID。客户端发送房间名称，服务端创建房间并返回 `room_id`。
- 请求体（JSON）：

```json
{
  "room_name": "string"
}
```

- 成功响应（JSON，HTTP 200）：

```json
{
  "room_id": "string"
}
```

- 失败响应（JSON，HTTP 400）：

```json
{
  "error": "请求参数无效|错误信息"
}
```

- 示例：

```bash
curl -X POST "http://localhost:8080/api/v1/rooms/create" \
  -H "Content-Type: application/json" \
  -d '{"room_name":"测试房间"}'
```

**关于加入房间（Join）**

- 注意：玩家加入房间的逻辑在代码中以 `JoinRoomRequest` 等 DTO 表示，但实际加入是通过 WebSocket 完成的（首次 WebSocket 消息为 `JoinGame`）。请参阅 WebSocket 接入说明：

- WebSocket 入口：`GET /ws/join`（即 `ws://<host>:<port>/api/v1/ws/join`）。
- 详情请参见：[docs/websock_dto.md](docs/websock_dto.md)

**错误与状态码约定（目前实现）**

- 参数解析失败或请求格式不合法：返回 HTTP 400，体为 `{ "error": "请求参数无效" }`。
- 业务错误（例如房间创建失败）：返回 HTTP 400，体为 `{ "error": "<具体错误信息>" }`。

**相关 DTO（简要）**

- `CreateRoomRequest`：`{ "room_name": "string" }`
- `CreateRoomResponse`：`{ "room_id": "string" }`
- `JoinRoomRequest`（仅作数据定义，实际通过 WS 使用）：`{ "room_id":"string", "joiner_name":"string" }`

如需我把 DTO 定义直接包含在文档中或添加更多示例（比如 Postman 集合），告诉我即可。
````
