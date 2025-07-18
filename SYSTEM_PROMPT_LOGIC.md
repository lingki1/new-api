# System Prompt Concatenation Logic

This document explains how the system combines system prompts from different sources before sending a request to an upstream Large Language Model (LLM) API.

## Goal

The primary goal is to allow administrators to define a global, channel-specific system prompt that is automatically prepended to the system prompt provided by the end-user in their API request. This ensures that all requests through a specific channel adhere to certain instructions or formats defined by the administrator.

## Storage of System Prompts

There are two sources for system prompts, and they are stored in different locations:

### 1. Channel System Prompt

This is the global, administrator-defined prompt for a specific channel.

-   **Storage Location (Database):** It is stored in the `channels` table in the database. In the code, this corresponds to the `SystemPrompt` field in the `model.Channel` struct.
    -   **File:** `model/channel.go`
    ```go
    type Channel struct {
        // ... other fields
        SystemPrompt      *string `json:"system_prompt" gorm:"type:text"`
    }
    ```
-   **How it's loaded:** When an incoming request is processed, the `distributor` middleware reads the channel's properties from the database and injects the channel's system prompt into the request context.
    -   **File:** `middleware/distributor.go`
    ```go
    // This line puts the channel's system prompt into the context for later use.
    c.Set("system_prompt", channel.GetSystemPrompt())
    ```

### 2. User Request System Prompt

This is the system prompt provided by the end-user in their specific API call.

-   **Storage Location (In-memory):** It is part of the JSON body of the user's HTTP request. According to the OpenAI API standard, it is the first message in the `messages` array with `"role": "system"`.
-   **How it's loaded:** The `getAndValidateTextRequest` function in `relay/relay-text.go` parses the user's request body into a `dto.GeneralOpenAIRequest` struct. The system prompt is accessible as the first element of the `textRequest.Messages` slice.

## Concatenation Logic

The core logic for combining these two prompts resides in the `TextHelper` function.

-   **File:** `relay/relay-text.go`

**Previous State (Incorrect):**
The code was attempting to access a non-existent field `relayInfo.ChannelSystemPrompt`, which caused a compilation error.

**Current State (Corrected Logic):**
The following logic has been implemented to correctly combine the prompts:

1.  The `GenRelayInfo` function first populates `relayInfo.SystemPrompt` with the **channel system prompt** that was loaded by the middleware.
2.  Then, inside `TextHelper`, before the request is sent to the upstream API, the code checks for a system prompt within the user's request (`textRequest.Messages`).
3.  The logic is as follows:
    -   **If both prompts exist:** The user's system prompt is appended to the channel's system prompt, separated by two newlines for clarity. The original user system prompt is then removed from the `messages` array to avoid duplication.
    -   **If only a user prompt exists:** It is moved into the `relayInfo.SystemPrompt` field and removed from the `messages` array.
    -   **If only a channel prompt exists:** Nothing needs to be done, as it's already in `relayInfo.SystemPrompt`.

This ensures that `relayInfo.SystemPrompt` always contains the final, combined system prompt, which is then used later in the request processing pipeline to construct the final message list sent to the upstream LLM.
