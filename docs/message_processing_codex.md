# Hướng dẫn Port Logic xử lý Message (Input/Output) cho Codex

Tài liệu này cung cấp chi tiết sự khác biệt giữa cấu trúc dữ liệu của **Claude** và **Codex**, kèm theo mã nguồn JavaScript hoàn chỉnh giúp bạn port logic xử lý input (request) và output (response stream) sang bất kỳ dự án nào sử dụng **Claude** làm backend và **Codex / Cursor / Droid CLI (OpenAI Responses API)** làm client.

---

## 1. So sánh Sự Khác Biệt Giữa Claude và Codex

### A. Định dạng Request Input (Đầu vào)

| Đặc tính | Claude (Anthropic Messages API) | Codex (OpenAI Responses API) |
| :--- | :--- | :--- |
| **System Prompt** | Nằm ở trường ngoài cùng: `"system": "..."` | Nằm ở trường ngoài cùng: `"instructions": "..."` |
| **Cấu trúc tin nhắn** | Sử dụng mảng `messages: [...]` lồng nhau. | Sử dụng mảng phẳng `input: [...]` chứa các khối riêng lẻ. |
| **Định dạng Text** | Tin nhắn có `role` và `content` là chuỗi hoặc mảng `{ type: "text", text: "..." }`. | Khối tin nhắn có `type: "message"`. Phần `content` chứa mảng `{ type: "input_text"/"output_text", text: "..." }`. |
| **Hình ảnh (Image)** | Dạng base64 nguồn lồng: `{ type: "image", source: { type: "base64", ... } }`. | Dạng URL hoặc ID: `{ type: "input_image", image_url: "..." }`. |
| **Tool Calls (Gọi Tool)** | Nằm trong `content` của lượt hội thoại `assistant` dạng `{ type: "tool_use", ... }`. | Là một phần tử riêng độc lập trong mảng `input` với `type: "function_call"`. |
| **Tool Outputs (Kết quả)** | Khối tin nhắn có role `"user"`, nội dung chứa `{ type: "tool_result", ... }`. | Là một phần tử riêng độc lập trong mảng `input` với `type: "function_call_output"`. |
| **Suy nghĩ (Thinking)** | Định cấu hình qua object `thinking: { type: "enabled", ... }`. | Định cấu hình qua `reasoning: { effort: "high/low" }` hoặc được lưu thành block riêng trong `input` có `type: "reasoning"`. |

#### Ví dụ So sánh JSON Request:

*   **Codex Request (Input)**:
    ```json
    {
      "instructions": "Bạn là trợ lý lập trình.",
      "input": [
        {
          "type": "message",
          "role": "user",
          "content": [{ "type": "input_text", "text": "Hãy gọi tool tính toán" }]
        },
        {
          "type": "function_call",
          "call_id": "call_123",
          "name": "calculator",
          "arguments": "{\"expression\":\"1+1\"}"
        },
        {
          "type": "function_call_output",
          "call_id": "call_123",
          "output": "2"
        }
      ]
    }
    ```

*   **Claude Request (Tương ứng sau khi chuyển đổi)**:
    ```json
    {
      "system": "Bạn là trợ lý lập trình.",
      "messages": [
        {
          "role": "user",
          "content": [{ "type": "text", "text": "Hãy gọi tool tính toán" }]
        },
        {
          "role": "assistant",
          "content": [
            {
              "type": "tool_use",
              "id": "call_123",
              "name": "calculator",
              "input": { "expression": "1+1" }
            }
          ]
        },
        {
          "role": "user",
          "content": [
            {
              "type": "tool_result",
              "tool_use_id": "call_123",
              "content": "2"
            }
          ]
        }
      ]
    }
    ```

---

### B. Định dạng Response Stream (Đầu ra SSE)

| Lượt Sự Kiện | Claude Stream Events | Codex Stream Events |
| :--- | :--- | :--- |
| **1. Khởi tạo** | `message_start` (Chứa thông tin model, ID tin nhắn ban đầu) | `response.created`<br>`response.in_progress` (Khởi tạo phiên trả lời của Codex) |
| **2. Bắt đầu block suy nghĩ (Thinking)** | `content_block_start` (type: `"thinking"`) | `response.output_item.added` (type: `"reasoning"`) <br>`response.reasoning_summary_part.added` |
| **3. Stream suy nghĩ** | `content_block_delta` (type: `"thinking_delta"`) | `response.reasoning_summary_text.delta` |
| **4. Kết thúc suy nghĩ** | `content_block_stop` | `response.reasoning_summary_text.done`<br>`response.reasoning_summary_part.done`<br>`response.output_item.done` |
| **5. Bắt đầu block văn bản (Text)** | `content_block_start` (type: `"text"`) | `response.output_item.added` (type: `"message"`, role: `"assistant"`) <br>`response.content_part.added` (type: `"output_text"`) |
| **6. Stream văn bản** | `content_block_delta` (type: `"text_delta"`) | `response.output_text.delta` |
| **7. Kết thúc văn bản** | `content_block_stop` | `response.output_text.done`<br>`response.content_part.done`<br>`response.output_item.done` |
| **8. Gọi Tool (nếu có)** | `content_block_start` (type: `"tool_use"`) | `response.output_item.added` (type: `"function_call"`) |
| **9. Stream tham số tool** | `content_block_delta` (type: `"input_json_delta"`) | `response.function_call_arguments.delta` |
| **10. Kết thúc gọi tool** | `content_block_stop` | `response.function_call_arguments.done`<br>`response.output_item.done` |
| **11. Hoàn tất** | `message_delta` (trả về stop_reason, token usage)<br>`message_stop` | `response.completed` (trả về usage, status)<br>`data: [DONE]` |

---

## 2. Mã nguồn chuyển đổi Request (Input từ Codex sang Claude)

```javascript
/**
 * Trình dịch Request: Codex (Responses API) -> Claude (Anthropic Messages API)
 */
function convertCodexRequestToClaude(codexBody) {
  const instructions = codexBody.instructions || "";
  const rawInput = codexBody.input || [];
  
  // 1. Chuẩn hóa input của Codex thành dạng mảng
  let inputItems = [];
  if (typeof rawInput === "string") {
    inputItems = [{
      type: "message",
      role: "user",
      content: [{ type: "input_text", text: rawInput.trim() || "..." }]
    }];
  } else if (Array.isArray(rawInput)) {
    inputItems = rawInput.length === 0 
      ? [{ type: "message", role: "user", content: [{ type: "input_text", text: "..." }] }]
      : rawInput;
  }

  const claudeMessages = [];
  let currentAssistantMsg = null;
  let pendingToolResults = [];

  // 2. Chuyển đổi từng block trong input thành messages chuẩn OpenAI/Claude
  for (const item of inputItems) {
    const itemType = item.type || (item.role ? "message" : null);

    if (itemType === "message") {
      // Flush assistant message cũ (nếu có)
      if (currentAssistantMsg) {
        claudeMessages.push(currentAssistantMsg);
        currentAssistantMsg = null;
      }
      // Flush tool results cũ
      if (pendingToolResults.length > 0) {
        claudeMessages.push(...pendingToolResults);
        pendingToolResults = [];
      }

      // Convert content: input_text/output_text -> text block
      const content = Array.isArray(item.content)
        ? item.content.map(c => {
            if (c.type === "input_text" || c.type === "output_text") {
              return { type: "text", text: c.text };
            }
            if (c.type === "input_image") {
              const base64Data = c.image_url || c.file_id || "";
              return {
                type: "image",
                source: {
                  type: "base64",
                  media_type: "image/jpeg",
                  data: base64Data.split(",")[1] || base64Data
                }
              };
            }
            return c;
          })
        : [{ type: "text", text: item.content || "" }];

      claudeMessages.push({ role: item.role, content });
    }
    else if (itemType === "function_call") {
      if (!currentAssistantMsg) {
        currentAssistantMsg = { role: "assistant", content: [] };
      }
      
      if (!item.name) continue;

      let parsedArgs = {};
      try {
        parsedArgs = typeof item.arguments === "string" 
          ? JSON.parse(item.arguments) 
          : item.arguments || {};
      } catch (e) {
        parsedArgs = {};
      }

      currentAssistantMsg.content.push({
        type: "tool_use",
        id: item.call_id,
        name: item.name,
        input: parsedArgs
      });
    }
    else if (itemType === "function_call_output") {
      if (currentAssistantMsg) {
        claudeMessages.push(currentAssistantMsg);
        currentAssistantMsg = null;
      }

      pendingToolResults.push({
        role: "user", // Claude yêu cầu tool_result phải có role: "user"
        content: [
          {
            type: "tool_result",
            tool_use_id: item.call_id,
            content: typeof item.output === "string" ? item.output : JSON.stringify(item.output)
          }
        ]
      });
    }
  }

  // Thu gom những phần còn lại
  if (currentAssistantMsg) {
    claudeMessages.push(currentAssistantMsg);
  }
  if (pendingToolResults.length > 0) {
    claudeMessages.push(...pendingToolResults);
  }

  // 3. Tạo body hoàn chỉnh cho Claude Messages API
  const claudeBody = {
    model: codexBody.model || "claude-3-7-sonnet-20250219",
    messages: claudeMessages,
    stream: true
  };

  if (instructions) {
    claudeBody.system = instructions;
  }

  if (codexBody.reasoning && typeof codexBody.reasoning === "object") {
    claudeBody.thinking = {
      type: "enabled",
      budget_tokens: codexBody.max_tokens ? Math.floor(codexBody.max_tokens * 0.8) : 2048
    };
  }

  return claudeBody;
}
```

---

## 3. Xử lý Output (Chuyển Claude Stream SSE sang Codex SSE)

### Trạng thái Stream (Stream State):
Mỗi request stream cần một object `state` độc lập:
```javascript
function createInitialStreamState(modelName) {
  return {
    seq: 0,
    responseId: `resp_${Math.random().toString(36).slice(2, 11)}`,
    created: Math.floor(Date.now() / 1000),
    started: false,
    msgTextBuf: {},
    msgItemAdded: {},
    msgContentAdded: {},
    msgItemDone: {},
    reasoningId: "",
    reasoningIndex: -1,
    reasoningBuf: "",
    reasoningDone: false,
    inThinking: false,
    funcArgsBuf: {},
    funcNames: {},
    funcCallIds: {},
    funcItemDone: {}
  };
}
```

### Hàm chuyển đổi Sự kiện Stream:

```javascript
/**
 * Dịch sự kiện SSE từ Claude sang Codex
 */
function translateClaudeSSEToCodex(claudeEvent, state) {
  const events = [];
  const nextSeq = () => ++state.seq;

  const emit = (eventType, data) => {
    data.sequence_number = nextSeq();
    events.push({ event: eventType, data });
  };

  const responseId = state.responseId;

  if (!state.started) {
    state.started = true;
    emit("response.created", {
      type: "response.created",
      response: {
        id: responseId,
        object: "response",
        created_at: state.created,
        status: "in_progress",
        background: false,
        error: null,
        output: []
      }
    });

    emit("response.in_progress", {
      type: "response.in_progress",
      response: {
        id: responseId,
        object: "response",
        created_at: state.created,
        status: "in_progress"
      }
    });
  }

  const type = claudeEvent.type;

  switch (type) {
    case "content_block_start": {
      const idx = claudeEvent.index;
      const block = claudeEvent.content_block;

      if (block.type === "thinking") {
        state.inThinking = true;
        state.reasoningId = `rs_${responseId}_${idx}`;
        state.reasoningIndex = idx;

        emit("response.output_item.added", {
          type: "response.output_item.added",
          output_index: idx,
          item: { id: state.reasoningId, type: "reasoning", summary: [] }
        });

        emit("response.reasoning_summary_part.added", {
          type: "response.reasoning_summary_part.added",
          item_id: state.reasoningId,
          output_index: idx,
          summary_index: 0,
          part: { type: "summary_text", text: "" }
        });
      }
      else if (block.type === "text") {
        const msgId = `msg_${responseId}_${idx}`;
        state.msgItemAdded[idx] = true;

        emit("response.output_item.added", {
          type: "response.output_item.added",
          output_index: idx,
          item: { id: msgId, type: "message", content: [], role: "assistant" }
        });

        emit("response.content_part.added", {
          type: "response.content_part.added",
          item_id: msgId,
          output_index: idx,
          content_index: 0,
          part: { type: "output_text", annotations: [], logprobs: [], text: "" }
        });
        state.msgContentAdded[idx] = true;
      }
      else if (block.type === "tool_use") {
        state.funcCallIds[idx] = block.id;
        state.funcNames[idx] = block.name;
        state.funcArgsBuf[idx] = "";

        emit("response.output_item.added", {
          type: "response.output_item.added",
          output_index: idx,
          item: {
            id: `fc_${block.id}`,
            type: "function_call",
            arguments: "",
            call_id: block.id,
            name: block.name
          }
        });
      }
      break;
    }

    case "content_block_delta": {
      const idx = claudeEvent.index;
      const delta = claudeEvent.delta;

      if (delta.type === "thinking_delta") {
        state.reasoningBuf += delta.thinking;
        emit("response.reasoning_summary_text.delta", {
          type: "response.reasoning_summary_text.delta",
          item_id: state.reasoningId,
          output_index: idx,
          summary_index: 0,
          delta: delta.thinking
        });
      }
      else if (delta.type === "text_delta") {
        const msgId = `msg_${responseId}_${idx}`;
        
        if (!state.msgItemAdded[idx]) {
          state.msgItemAdded[idx] = true;
          emit("response.output_item.added", {
            type: "response.output_item.added",
            output_index: idx,
            item: { id: msgId, type: "message", content: [], role: "assistant" }
          });
        }
        if (!state.msgContentAdded[idx]) {
          emit("response.content_part.added", {
            type: "response.content_part.added",
            item_id: msgId,
            output_index: idx,
            content_index: 0,
            part: { type: "output_text", annotations: [], logprobs: [], text: "" }
          });
          state.msgContentAdded[idx] = true;
        }

        state.msgTextBuf[idx] = (state.msgTextBuf[idx] || "") + delta.text;

        emit("response.output_text.delta", {
          type: "response.output_text.delta",
          item_id: msgId,
          output_index: idx,
          content_index: 0,
          delta: delta.text,
          logprobs: []
        });
      }
      else if (delta.type === "input_json_delta") {
        const callId = state.funcCallIds[idx];
        state.funcArgsBuf[idx] += delta.partial_json;

        emit("response.function_call_arguments.delta", {
          type: "response.function_call_arguments.delta",
          item_id: `fc_${callId}`,
          output_index: idx,
          delta: delta.partial_json
        });
      }
      break;
    }

    case "content_block_stop": {
      const idx = claudeEvent.index;

      if (state.inThinking && idx === state.reasoningIndex) {
        state.inThinking = false;
        state.reasoningDone = true;

        emit("response.reasoning_summary_text.done", {
          type: "response.reasoning_summary_text.done",
          item_id: state.reasoningId,
          output_index: idx,
          summary_index: 0,
          text: state.reasoningBuf
        });

        emit("response.reasoning_summary_part.done", {
          type: "response.reasoning_summary_part.done",
          item_id: state.reasoningId,
          output_index: idx,
          summary_index: 0,
          part: { type: "summary_text", text: state.reasoningBuf }
        });

        emit("response.output_item.done", {
          type: "response.output_item.done",
          output_index: idx,
          item: {
            id: state.reasoningId,
            type: "reasoning",
            summary: [{ type: "summary_text", text: state.reasoningBuf }]
          }
        });
      }
      else if (state.msgItemAdded[idx] && !state.msgItemDone[idx]) {
        state.msgItemDone[idx] = true;
        const msgId = `msg_${responseId}_${idx}`;
        const fullText = state.msgTextBuf[idx] || "";

        emit("response.output_text.done", {
          type: "response.output_text.done",
          item_id: msgId,
          output_index: idx,
          content_index: 0,
          text: fullText,
          logprobs: []
        });

        emit("response.content_part.done", {
          type: "response.content_part.done",
          item_id: msgId,
          output_index: idx,
          content_index: 0,
          part: { type: "output_text", annotations: [], logprobs: [], text: fullText }
        });

        emit("response.output_item.done", {
          type: "response.output_item.done",
          output_index: idx,
          item: {
            id: msgId,
            type: "message",
            content: [{ type: "output_text", annotations: [], logprobs: [], text: fullText }],
            role: "assistant"
          }
        });
      }
      else if (state.funcCallIds[idx] && !state.funcItemDone[idx]) {
        state.funcItemDone[idx] = true;
        const callId = state.funcCallIds[idx];
        const args = state.funcArgsBuf[idx] || "{}";

        emit("response.function_call_arguments.done", {
          type: "response.function_call_arguments.done",
          item_id: `fc_${callId}`,
          output_index: idx,
          arguments: args
        });

        emit("response.output_item.done", {
          type: "response.output_item.done",
          output_index: idx,
          item: {
            id: `fc_${callId}`,
            type: "function_call",
            arguments: args,
            call_id: callId,
            name: state.funcNames[idx]
          }
        });
      }
      break;
    }

    case "message_stop": {
      emit("response.completed", {
        type: "response.completed",
        response: {
          id: responseId,
          object: "response",
          created_at: state.created,
          status: "completed",
          background: false,
          error: null
        }
      });
      break;
    }
  }

  return events;
}

/**
 * Thu dọn và hoàn tất tất cả các block chưa được đóng
 */
function flushRemainingEvents(state) {
  const events = [];
  const nextSeq = () => ++state.seq;
  const emit = (eventType, data) => {
    data.sequence_number = nextSeq();
    events.push({ event: eventType, data });
  };

  const responseId = state.responseId;

  for (const idx in state.msgItemAdded) {
    if (!state.msgItemDone[idx]) {
      state.msgItemDone[idx] = true;
      const msgId = `msg_${responseId}_${idx}`;
      const fullText = state.msgTextBuf[idx] || "";

      emit("response.output_text.done", {
        type: "response.output_text.done",
        item_id: msgId,
        output_index: parseInt(idx),
        content_index: 0,
        text: fullText,
        logprobs: []
      });

      emit("response.content_part.done", {
        type: "response.content_part.done",
        item_id: msgId,
        output_index: parseInt(idx),
        content_index: 0,
        part: { type: "output_text", annotations: [], logprobs: [], text: fullText }
      });

      emit("response.output_item.done", {
        type: "response.output_item.done",
        output_index: parseInt(idx),
        item: {
          id: msgId,
          type: "message",
          content: [{ type: "output_text", annotations: [], logprobs: [], text: fullText }],
          role: "assistant"
        }
      });
    }
  }

  for (const idx in state.funcCallIds) {
    if (!state.funcItemDone[idx]) {
      state.funcItemDone[idx] = true;
      const callId = state.funcCallIds[idx];
      const args = state.funcArgsBuf[idx] || "{}";

      emit("response.function_call_arguments.done", {
        type: "response.function_call_arguments.done",
        item_id: `fc_${callId}`,
        output_index: parseInt(idx),
        arguments: args
      });

      emit("response.output_item.done", {
        type: "response.output_item.done",
        output_index: parseInt(idx),
        item: {
          id: `fc_${callId}`,
          type: "function_call",
          arguments: args,
          call_id: callId,
          name: state.funcNames[idx]
        }
      });
    }
  }

  emit("response.completed", {
    type: "response.completed",
    response: {
      id: responseId,
      object: "response",
      created_at: state.created,
      status: "completed",
      background: false,
      error: null
    }
  });

  return events;
}
```

---

## 4. Middleware Route HTTP Tích Hợp SSE (Express/NestJS)

```javascript
app.post("/v1/responses", async (req, res) => {
  res.writeHead(200, {
    "Content-Type": "text/event-stream",
    "Cache-Control": "no-cache",
    "Connection": "keep-alive"
  });

  const claudePayload = convertCodexRequestToClaude(req.body);
  const streamState = createInitialStreamState(claudePayload.model);

  try {
    const responseStream = await anthropic.messages.create(claudePayload);

    for await (const chunk of responseStream) {
      const codexEvents = translateClaudeSSEToCodex(chunk, streamState);
      for (const ev of codexEvents) {
        res.write(`event: ${ev.event}\n`);
        res.write(`data: ${JSON.stringify(ev.data)}\n\n`);
      }
    }

    const remainingEvents = flushRemainingEvents(streamState);
    for (const ev of remainingEvents) {
      res.write(`event: ${ev.event}\n`);
      res.write(`data: ${JSON.stringify(ev.data)}\n\n`);
    }

    res.write("data: [DONE]\n\n");
    res.end();
  } catch (error) {
    const errEvent = {
      type: "response.failed",
      sequence_number: ++streamState.seq,
      response: {
        id: streamState.responseId,
        object: "response",
        status: "failed",
        error: {
          message: error.message || "Internal Server Error",
          type: "invalid_request_error"
        }
      }
    };
    res.write(`event: response.failed\n`);
    res.write(`data: ${JSON.stringify(errEvent)}\n\n`);
    res.end();
  }
});
```
