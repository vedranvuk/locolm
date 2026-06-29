# llama.cpp — AI Development Reference

> Compiled from official documentation (github.com/ggml-org/llama.cpp, 2026-06-29).
> This file is the single source of truth for AI-driven development against llama.cpp.

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Build System](#build-system)
4. [Executables / Tools](#executables--tools)
5. [C API (llama.h)](#c-api-llamah)
6. [llama-cli](#llama-cli)
7. [llama-server](#llama-server)
8. [REST API Endpoints](#rest-api-endpoints)
9. [Quantization](#quantization)
10. [GBNF Grammars](#gbnf-grammars)
11. [GGUF Format](#gguf-format)
12. [Key Data Structures](#key-data-structures)
13. [Environment Variables](#environment-variables)
14. [Model Sources](#model-sources)
15. [Multi-GPU & Backend Notes](#multi-gpu--backend-notes)

---

## Overview

**llama.cpp** is a plain C/C++ implementation of LLM inference with minimal dependencies. It runs on CPU and GPU (CUDA, Metal, Vulkan, HIP, SYCL, OpenCL, CANN, WebGPU, etc.).

- **License:** MIT
- **Languages:** C++ (57%), C (14%), Python (7.5%), CUDA (5.4%), TypeScript (3.6%)
- **Repo:** https://github.com/ggml-org/llama.cpp
- **Core library:** `libllama` (C API in `include/llama.h`)
- **Quantization format:** GGUF (GGML Universal File)

### Key Capabilities
- 1.5-bit to 8-bit integer quantization
- CPU+GPU hybrid inference
- AVX/AVX2/AVX512/AMX (x86), ARM NEON (ARM), RVV (RISC-V)
- Speculative decoding (draft models, n-gram, EAGLE, MTP)
- LoRA adapters, control vectors
- Multimodal (vision/audio) via MTMD subsystem
- Embedding extraction, reranking
- Flash Attention, KV cache quantization
- Continuous batching, multi-user parallel decoding

---

## Architecture

```
llama.cpp/
├── include/
│   └── llama.h              # Public C API
├── src/                     # Core library implementation
├── ggml/                    # Tensor library (ggml)
│   └── src/
│       ├── ggml-cuda/       # CUDA backend
│       ├── ggml-metal/      # Metal backend
│       ├── ggml-vulkan/     # Vulkan backend
│       └── ...              # Other backends
├── common/                  # Shared utilities
├── tools/
│   ├── cli/                 # llama-cli
│   ├── server/              # llama-server
│   ├── quantize/            # llama-quantize
│   ├── llama-bench/         # llama-bench
│   ├── perplexity/          # llama-perplexity
│   ├── completion/          # llama-completion
│   ├── mtmd/                # Multimodal subsystem
│   └── ui/                  # Web UI
├── examples/                # Usage examples
├── grammars/                # GBNF grammar files
├── conversion/              # Model conversion scripts
└── docs/                    # Documentation
```

### Library Stack
```
Application (llama-cli, llama-server, custom)
    │
    ▼
libllama (include/llama.h)
    │
    ▼
ggml (tensor computation engine)
    │
    ▼
Backends (CPU, CUDA, Metal, Vulkan, HIP, SYCL, OpenCL, CANN, WebGPU, RPC)
```

---

## Build System

Uses **CMake**. Basic build:

```bash
cmake -B build
cmake --build build --config Release
```

### Build Targets
| Target | Description |
|--------|-------------|
| `llama-cli` | Interactive CLI tool |
| `llama-server` | HTTP server |
| `llama-quantize` | Model quantization |
| `llama-bench` | Benchmarking |
| `llama-perplexity` | Perplexity measurement |
| `llama-simple` | Minimal example |
| `llama-completion` | Shell completion helper |

### Backend CMake Flags
| Flag | Backend |
|------|---------|
| `-DGGML_CUDA=ON` | NVIDIA GPU |
| `-DGGML_METAL=ON` | Apple Metal (default on macOS) |
| `-DGGML_VULKAN=ON` | Vulkan |
| `-DGGML_HIP=ON` | AMD GPU (ROCm) |
| `-DGGML_MUSA=ON` | Moore Threads GPU |
| `-DGGML_SYCL=ON` | Intel GPU |
| `-DGGML_OPENCL=ON` | OpenCL (Adreno) |
| `-DGGML_CANN=ON` | Ascend NPU |
| `-DGGML_WEBGPU=ON` | WebGPU |
| `-DGGML_OPENMP=ON` | OpenMP CPU parallel |
| `-DGGML_BLAS=ON` | BLAS acceleration |
| `-DGGML_ZENDNN=ON` | AMD CPU (ZenDNN) |
| `-DGGML_CPU_KLEIDIAI=ON` | Arm KleidiAI |

### Useful Build Options
- `-DBUILD_SHARED_LIBS=OFF` — Static build
- `-DGGML_NATIVE=OFF` — Build for all GPU architectures (non-native)
- `-DLLAMA_OPENSSL=ON` — SSL support for server
- `-j 8` — Parallel compilation
- `ccache` — Faster repeated builds

---

## Executables / Tools

### llama-cli
Interactive CLI for text generation, conversation, and model testing.

```bash
# Basic usage
llama-cli -m model.gguf -p "Hello, how are you?"

# Conversation mode
llama-cli -m model.gguf -cnv

# Download from Hugging Face
llama-cli -hf ggml-org/gemma-3-1b-it-GGUF

# With GPU offload
llama-cli -m model.gguf -ngl 99

# With grammar constraint
llama-cli -m model.gguf --grammar-file grammars/json.gbnf

# Multimodal
llama-cli -m model.gguf --mmproj mmproj.gguf --image photo.jpg -p "Describe this"
```

### llama-server
OpenAI-compatible HTTP server.

```bash
llama-server -m model.gguf --port 8080
llama-server -hf ggml-org/gemma-3-1b-it-GGUF
llama-server -m model.gguf --host 0.0.0.0 --port 8080 -ngl 99
```

### llama-quantize
Quantize GGUF models.

```bash
./build/bin/llama-quantize input.gguf output-Q4_K_M.gguf Q4_K_M
./build/bin/llama-quantize --imatrix imatrix.gguf input.gguf output.gguf Q4_K_M
```

### llama-bench
Benchmark inference performance.

```bash
llama-bench -m model.gguf
```

### llama-perplexity
Measure model quality.

```bash
llama-perplexity -m model.gguf -f test.txt
```

---

## C API (llama.h)

The public C API is defined in `include/llama.h`. Key functions:

### Initialization
```c
void llama_backend_init(void);
void llama_backend_free(void);
void llama_numa_init(enum ggml_numa_strategy numa);
```

### Model Loading
```c
struct llama_model * llama_model_load_from_file(const char * path, struct llama_model_params params);
struct llama_model * llama_model_load_from_splits(const char ** paths, size_t n_paths, struct llama_model_params params);
void llama_model_free(struct llama_model * model);
```

### Context
```c
struct llama_context * llama_init_from_model(struct llama_model * model, struct llama_context_params params);
void llama_free(struct llama_context * ctx);
```

### Decoding
```c
int32_t llama_encode(struct llama_context * ctx, struct llama_batch batch);
int32_t llama_decode(struct llama_context * ctx, struct llama_batch batch);
```

### Tokenization
```c
int32_t llama_tokenize(const struct llama_vocab * vocab, const char * text, int32_t text_len, llama_token * tokens, int32_t n_tokens_max, bool add_special, bool parse_special);
int32_t llama_token_to_piece(const struct llama_vocab * vocab, llama_token token, char * buf, int32_t length, int32_t lstrip, bool special);
int32_t llama_detokenize(const struct llama_vocab * vocab, const llama_token * tokens, int32_t n_tokens, char * text, int32_t text_len_max, bool remove_special, bool unparse_special);
```

### Sampling
```c
struct llama_sampler * llama_sampler_chain_init(struct llama_sampler_chain_params params);
void llama_sampler_chain_add(struct llama_sampler * chain, struct llama_sampler * smpl);
llama_token llama_sampler_sample(struct llama_sampler * smpl, struct llama_context * ctx, int32_t idx);
void llama_sampler_accept(struct llama_sampler * smpl, llama_token token);
void llama_sampler_free(struct llama_sampler * smpl);
```

### Sampler Types
```c
llama_sampler_init_greedy()
llama_sampler_init_dist(uint32_t seed)
llama_sampler_init_top_k(int32_t k)
llama_sampler_init_top_p(float p, size_t min_keep)
llama_sampler_init_min_p(float p, size_t min_keep)
llama_sampler_init_typical(float p, size_t min_keep)
llama_sampler_init_temp(float t)
llama_sampler_init_temp_ext(float t, float delta, float exponent)
llama_sampler_init_xtc(float p, float t, size_t min_keep, uint32_t seed)
llama_sampler_init_mirostat(int32_t n_vocab, uint32_t seed, float tau, float eta, int32_t m)
llama_sampler_init_mirostat_v2(uint32_t seed, float tau, float eta)
llama_sampler_init_grammar(const struct llama_vocab * vocab, const char * grammar_str, const char * grammar_root)
llama_sampler_init_penalties(int32_t penalty_last_n, float penalty_repeat, float penalty_freq, float penalty_present)
llama_sampler_init_dry(const struct llama_vocab * vocab, int32_t n_ctx_train, float dry_multiplier, float dry_base, int32_t dry_allowed_length, int32_t dry_penalty_last_n, const char ** seq_breakers, size_t num_breakers)
llama_sampler_init_logit_bias(int32_t n_vocab, int32_t n_logit_bias, const llama_logit_bias * logit_bias)
llama_sampler_init_infill(const struct llama_vocab * vocab)
llama_sampler_init_adaptive_p(float target, float decay, uint32_t seed)
llama_sampler_init_top_n_sigma(float n)
```

### Chat Templates
```c
int32_t llama_chat_apply_template(const char * tmpl, const struct llama_chat_message * chat, size_t n_msg, bool add_ass, char * buf, int32_t length);
int32_t llama_chat_builtin_templates(const char ** output, size_t len);
```

### LoRA Adapters
```c
struct llama_adapter_lora * llama_adapter_lora_init(struct llama_model * model, const char * path);
int32_t llama_set_adapters_lora(struct llama_context * ctx, struct llama_adapter_lora ** adapters, size_t n_adapters, float * scales);
void llama_adapter_lora_free(struct llama_adapter_lora * adapter);
```

### Memory Management
```c
void llama_memory_clear(llama_memory_t mem, bool data);
bool llama_memory_seq_rm(llama_memory_t mem, llama_seq_id seq_id, llama_pos p0, llama_pos p1);
void llama_memory_seq_cp(llama_memory_t mem, llama_seq_id seq_id_src, llama_seq_id seq_id_dst, llama_pos p0, llama_pos p1);
void llama_memory_seq_keep(llama_memory_t mem, llama_seq_id seq_id);
void llama_memory_seq_add(llama_memory_t mem, llama_seq_id seq_id, llama_pos p0, llama_pos p1, llama_pos delta);
void llama_memory_seq_div(llama_memory_t mem, llama_seq_id seq_id, llama_pos p0, llama_pos p1, int d);
llama_pos llama_memory_seq_pos_min(llama_memory_t mem, llama_seq_id seq_id);
llama_pos llama_memory_seq_pos_max(llama_memory_t mem, llama_seq_id seq_id);
bool llama_memory_can_shift(llama_memory_t mem);
```

### State Persistence
```c
size_t llama_state_get_size(struct llama_context * ctx);
size_t llama_state_get_data(struct llama_context * ctx, uint8_t * dst, size_t size);
size_t llama_state_set_data(struct llama_context * ctx, const uint8_t * src, size_t size);
bool llama_state_load_file(struct llama_context * ctx, const char * filepath);
bool llama_state_save_file(struct llama_context * ctx, const char * filepath);
```

### Logits & Embeddings
```c
float * llama_get_logits(struct llama_context * ctx);
float * llama_get_logits_ith(struct llama_context * ctx, int32_t i);
float * llama_get_embeddings(struct llama_context * ctx);
float * llama_get_embeddings_ith(struct llama_context * ctx, int32_t i);
float * llama_get_embeddings_seq(struct llama_context * ctx, llama_seq_id seq_id);
```

### Model Introspection
```c
int32_t llama_model_n_ctx_train(const struct llama_model * model);
int32_t llama_model_n_embd(const struct llama_model * model);
int32_t llama_model_n_layer(const struct llama_model * model);
int32_t llama_model_n_head(const struct llama_model * model);
int32_t llama_model_n_head_kv(const struct llama_model * model);
int32_t llama_vocab_n_tokens(const struct llama_vocab * vocab);
uint64_t llama_model_n_params(const struct llama_model * model);
uint64_t llama_model_size(const struct llama_model * model);
bool llama_model_has_encoder(const struct llama_model * model);
bool llama_model_has_decoder(const struct llama_model * model);
bool llama_model_is_recurrent(const struct llama_model * model);
bool llama_model_is_hybrid(const struct llama_model * model);
bool llama_model_is_diffusion(const struct llama_model * model);
const char * llama_model_chat_template(const struct llama_model * model, const char * name);
```

### Performance
```c
struct llama_perf_context_data llama_perf_context(const struct llama_context * ctx);
void llama_perf_context_print(const struct llama_context * ctx);
void llama_perf_context_reset(struct llama_context * ctx);
struct llama_perf_sampler_data llama_perf_sampler(const struct llama_sampler * chain);
void llama_perf_sampler_print(const struct llama_sampler * chain);
void llama_perf_sampler_reset(struct llama_sampler * chain);
```

---

## llama-cli

### Common Parameters
| Flag | Description |
|------|-------------|
| `-m, --model FNAME` | Model path |
| `-hf, --hf-repo <user>/<model>[:quant]` | Hugging Face model |
| `-p, --prompt PROMPT` | Prompt text |
| `-f, --file FNAME` | Prompt from file |
| `-sys, --system-prompt PROMPT` | System prompt |
| `-c, --ctx-size N` | Context size (0 = from model) |
| `-n, --predict N` | Tokens to predict (-1 = infinity) |
| `-t, --threads N` | CPU threads |
| `-ngl, --gpu-layers N` | GPU layers (auto/all/exact) |
| `-sm, --split-mode {none,layer,row,tensor}` | Multi-GPU split |
| `-ts, --tensor-split N0,N1,...` | Per-GPU split fractions |
| `-b, --batch-size N` | Logical batch size |
| `-ub, --ubatch-size N` | Physical batch size |
| `--mmproj FILE` | Multimodal projector |
| `--image, --audio, --video FILE` | Multimodal input |
| `--lora FNAME` | LoRA adapter |
| `--lora-scaled FNAME:SCALE` | LoRA with custom scale |
| `--control-vector FNAME` | Control vector |
| `-dev, --device <dev1,dev2,...>` | Device list |
| `--list-devices` | List available devices |

### Sampling Parameters
| Flag | Default | Description |
|------|---------|-------------|
| `--temp, --temperature N` | 0.80 | Temperature |
| `--top-k N` | 40 | Top-K sampling |
| `--top-p N` | 0.95 | Top-P (nucleus) sampling |
| `--min-p N` | 0.05 | Min-P sampling |
| `--top-nsigma N` | -1.0 | Top-n-sigma sampling |
| `--typical, --typical-p N` | 1.0 | Locally typical sampling |
| `--xtc-probability N` | 0.0 | XTC probability |
| `--xtc-threshold N` | 0.1 | XTC threshold |
| `--repeat-penalty N` | 1.0 | Repeat penalty |
| `--repeat-last-n N` | 64 | Repeat lookback |
| `--presence-penalty N` | 0.0 | Presence penalty |
| `--frequency-penalty N` | 0.0 | Frequency penalty |
| `--dry-multiplier N` | 0.0 | DRY multiplier |
| `--dry-base N` | 1.75 | DRY base |
| `--mirostat N` | 0 | Mirostat (0=off, 1=v1, 2=v2) |
| `--mirostat-lr N` | 0.1 | Mirostat learning rate |
| `--mirostat-ent N` | 5.0 | Mirostat target entropy |
| `--dynatemp-range N` | 0.0 | Dynamic temp range |
| `--dynatemp-exp N` | 1.0 | Dynamic temp exponent |
| `-l, --logit-bias TOKEN_ID(+/-)BIAS` | | Logit bias |
| `--grammar GRAMMAR` | | GBNF grammar string |
| `--grammar-file FNAME` | | GBNF grammar file |
| `-j, --json-schema SCHEMA` | | JSON schema constraint |
| `--samplers SAMPLERS` | penalties;dry;top_n_sigma;top_k;typ_p;top_p;min_p;xtc;temperature | Sampler order |

### CLI-Specific Parameters
| Flag | Description |
|------|-------------|
| `-cnv, --conversation` | Conversation mode |
| `-st, --single-turn` | Single turn only |
| `-mli, --multiline-input` | Multi-line input |
| `-r, --reverse-prompt PROMPT` | Stop at prompt |
| `-sp, --special` | Show special tokens |
| `-e, --escape` | Process escape sequences |
| `--color [on|off|auto]` | Colored output |
| `--warmup` | Warmup run |
| `--jinja / --no-jinja` | Jinja template engine |
| `--chat-template NAME` | Custom chat template |
| `--reasoning [on|off|auto]` | Reasoning/thinking mode |
| `--reasoning-format FORMAT` | Reasoning format (none/deepseek/deepseek-legacy) |
| `--reasoning-budget N` | Token budget for thinking |
| `--spec-type TYPE` | Speculative decoding type |
| `--spec-draft-model FNAME` | Draft model for speculation |
| `--spec-draft-n-max N` | Max draft tokens |

### Built-in Chat Templates
bailing, bailing-think, bailing2, chatglm3, chatglm4, chatml, command-r, deepseek, deepseek-ocr, deepseek2, deepseek3, exaone-moe, exaone3, exaone4, falcon3, gemma, gigachat, glmedge, gpt-oss, granite, granite-4.0, granite-4.1, grok-2, hunyuan-dense, hunyuan-moe, hunyuan-vl, kimi-k2, llama2, llama2-sys, llama2-sys-bos, llama2-sys-strip, llama3, llama4, megrez, phi4, qwen2, qwen3, ...

---

## llama-server

### Server-Specific Parameters
| Flag | Default | Description |
|------|---------|-------------|
| `--host HOST` | 127.0.0.1 | Bind address |
| `--port PORT` | 8080 | Listen port |
| `--reuse-port` | | Allow port reuse |
| `--path PATH` | | Static files path |
| `--api-prefix PREFIX` | | API path prefix |
| `-np, --parallel N` | auto | Server slots |
| `-cb, --cont-batching` | enabled | Continuous batching |
| `--cache-prompt` | enabled | Prompt caching |
| `--cache-reuse N` | 0 | KV cache reuse chunk size |
| `-cram, --cache-ram MiB` | 8192 | Max cache RAM |
| `-kvu, --kv-unified` | enabled | Unified KV buffer |
| `--embedding` | | Embedding-only mode |
| `--rerank` | | Enable reranking |
| `--pooling {none,mean,cls,last,rank}` | | Pooling type |
| `--embd-normalize N` | 2 | Embedding normalization |
| `--metrics` | | Prometheus metrics |
| `--slots` | enabled | Slots monitoring |
| `--props` | | Allow POST /props |
| `--api-key KEY` | | API key auth |
| `--ssl-key-file FNAME` | | SSL key |
| `--ssl-cert-file FNAME` | | SSL cert |
| `--jinja` | enabled | Jinja templates |
| `--chat-template NAME` | | Custom chat template |
| `--reasoning [on|off|auto]` | auto | Reasoning mode |
| `--reasoning-budget N` | -1 | Thinking token budget |
| `--tools TOOL1,TOOL2,...` | | Built-in tools |
| `--agent` | | Enable agent mode (tools + CORS) |
| `--media-path PATH` | | Local media directory |
| `--models-dir PATH` | | Models directory (router mode) |
| `--models-preset PATH` | | Models preset INI file |
| `--models-max N` | 4 | Max simultaneous models |
| `--sleep-idle-seconds SECONDS` | -1 | Auto-sleep timeout |
| `-to, --timeout N` | 3600 | Read/write timeout |
| `--threads-http N` | -1 | HTTP processing threads |
| `--sse-ping-interval N` | 30 | SSE ping interval |
| `--ui / --no-ui` | enabled | Web UI |
| `--ui-config JSON` | | Default UI settings |

### Built-in Tools (for agent mode)
`read_file`, `file_glob_search`, `grep_search`, `exec_shell_command`, `write_file`, `edit_file`, `apply_diff`, `get_datetime`

---

## REST API Endpoints

### Non-OpenAI Endpoints

#### `GET /health`
Health check. Returns 200 `{"status": "ok"}` or 503 if loading.

#### `POST /completion`
Text completion. Key fields:
- `prompt` (string|array) — Input text or tokens
- `n_predict` (int) — Max tokens (default: -1 = infinity)
- `temperature`, `top_k`, `top_p`, `min_p` — Sampling
- `stream` (bool) — SSE streaming
- `stop` (string[]) — Stop sequences
- `grammar` (string) — GBNF grammar
- `json_schema` (object) — JSON schema constraint
- `seed` (int) — RNG seed
- `ignore_eos` (bool) — Ignore EOS token
- `logit_bias` (array) — Token bias `[[id, bias], ...]`
- `n_probs` (int) — Return top-N probabilities
- `cache_prompt` (bool) — Reuse KV cache
- `id_slot` (int) — Assign to slot
- `lora` (array) — Per-request LoRA: `[{"id": 0, "scale": 0.5}]`
- `response_fields` (string[]) — Select response fields

#### `POST /tokenize`
Tokenize text. Fields: `content`, `add_special`, `parse_special`, `with_pieces`.

#### `POST /detokenize`
Convert tokens to text. Fields: `tokens`.

#### `POST /apply-template`
Apply chat template to messages. Returns formatted prompt string.

#### `POST /embedding`
Generate embeddings. Fields: `content`, `embd_normalize`.

#### `POST /reranking`
Rerank documents. Fields: `query`, `documents`, `top_n`.

#### `POST /infill`
Code infill. Fields: `input_prefix`, `input_suffix`, `input_extra`, `prompt`.

#### `GET /props`
Server properties and default generation settings.

#### `POST /props`
Modify global properties (requires `--props` flag).

#### `GET /slots`
Slot monitoring (enabled by default).

#### `GET /metrics`
Prometheus metrics (requires `--metrics` flag).

#### `GET /lora-adapters`
List loaded LoRA adapters.

#### `POST /lora-adapters`
Set LoRA adapter scales.

#### `POST /slots/{id}?action=save`
Save slot KV cache.

#### `POST /slots/{id}?action=restore`
Restore slot KV cache.

#### `POST /slots/{id}?action=erase`
Erase slot KV cache.

### OpenAI-Compatible Endpoints

#### `GET /v1/models`
List loaded models.

#### `POST /v1/completions`
OpenAI Completions API. Supports `prompt`, `max_tokens`, `temperature`, `stream`, etc.

#### `POST /v1/chat/completions`
OpenAI Chat Completions API. Key fields:
- `model` — Model name/alias
- `messages` — Chat messages `[{"role", "content"}]`
- `max_tokens` — Max generation tokens
- `temperature`, `top_p`, `top_k` — Sampling
- `stream` — SSE streaming
- `response_format` — JSON output: `{"type": "json_object"}` or `{"type": "json_schema", "schema": {...}}`
- `tools` — Tool definitions
- `tool_choice` — Tool selection mode
- `chat_template_kwargs` — Extra template params
- `reasoning_format` — Reasoning format
- `reasoning_control` — Enable real-time reasoning control
- `parallel_tool_calls` — Parallel tool calls

#### `POST /v1/chat/completions/control`
Control in-flight completion. Fields: `id`, `action` (currently only `reasoning_end`).

#### `POST /v1/responses`
OpenAI Responses API. Converts to chat completions internally.

#### `POST /v1/embeddings`
OpenAI Embeddings API. Fields: `input`, `model`, `encoding_format`.

#### `POST /v1/responses/input_tokens`
Token counting for Responses API.

#### `POST /v1/chat/completions/input_tokens`
Token counting for Chat Completions API.

### Anthropic-Compatible Endpoints

#### `POST /v1/messages`
Anthropic Messages API. Fields: `model`, `messages`, `max_tokens`, `system`, `temperature`, `top_p`, `top_k`, `stop_sequences`, `stream`, `tools`, `tool_choice`.

#### `POST /v1/messages/count_tokens`
Token counting for Messages API.

### Router Mode Endpoints (Multi-Model)

#### `GET /models`
List all available models with status.

#### `POST /models/load`
Load a model. Body: `{"model": "..."}`.

#### `POST /models/unload`
Unload a model.

#### `POST /models`
Download a model.

#### `DELETE /models?model=...`
Delete model from cache.

#### `GET /models/sse`
Real-time model status events.

---

## Quantization

### Quantization Types (llama_ftype enum)
| Type | Bits | Description |
|------|------|-------------|
| `Q4_0` | 4-bit | Original quant |
| `Q4_1` | 4-bit | Higher precision |
| `Q5_0` | 5-bit | |
| `Q5_1` | 5-bit | |
| `Q8_0` | 8-bit | Near-lossless |
| `Q2_K` | 2-bit | K-quant |
| `Q3_K_S/M/L` | 3-bit | K-quant variants |
| `Q4_K_S/M` | 4-bit | K-quant (recommended) |
| `Q5_K_S/M` | 5-bit | K-quant |
| `Q6_K` | 6-bit | K-quant |
| `IQ4_NL` | 4-bit | I-quant (non-linear) |
| `TQ1_0` | 1.5-bit | Ternary |
| `TQ2_0` | 2-bit | Ternary |
| `MXFP4_MOE` | 4-bit | For MoE experts |
| `NVFP4` | 4-bit | NVIDIA FP4 |
| `Q1_0` | 1-bit | Experimental |
| `BF16` | 16-bit | Brain float |
| `F16` | 16-bit | Half precision |
| `F32` | 32-bit | Full precision |

### Quantize Command
```bash
./build/bin/llama-quantize input.gguf output.gguf <type> [threads]
```

### Quantize Options
- `--allow-requantize` — Re-quantize already-quantized tensors
- `--leave-output-tensor` — Keep output.weight unquantized
- `--pure` — Disable k-quant mixtures
- `--imatrix FILE` — Importance matrix
- `--include-weights tensor` — Apply imatrix to specific tensors
- `--exclude-weights tensor` — Exclude tensors from imatrix
- `--output-tensor-type TYPE` — Output tensor quant type
- `--token-embedding-type TYPE` — Token embedding quant type
- `--keep-split` — Preserve shard structure
- `--tensor-type PATTERN=TYPE` — Per-tensor quant (regex)
- `--prune-layers N,M,...` — Remove layers
- `--override-kv KEY=TYPE:VALUE` — Override metadata

### Conversion from Hugging Face
```bash
python convert_hf_to_gguf.py --outfile model.gguf --outtype bf16 --remote org/model
```

---

## GBNF Grammars

GBNF (GGML BNF) constrains model output to a formal grammar.

### Syntax
```
# Comments use #
root ::= expression
rule ::= terminals | non-terminals

# Terminals: characters or ranges
digit ::= [0-9]
letter ::= [a-zA-Z]

# Sequences
name ::= letter (letter | digit)*

# Alternatives
color ::= "red" | "green" | "blue"

# Repetition
*  — zero or more
+  — one or more
?  — optional
{m} — exactly m
{m,} — at least m
{m,n} — between m and n

# Tokens (match specific tokenizer tokens)
<[123]>     — token by ID
<think>     — token by text
!<[100]>    — negated token
```

### JSON Schema Conversion
llama.cpp converts JSON Schema to GBNF automatically:
- CLI: `-j '{...}'` or `--json-schema-file FILE`
- Server: `json_schema` field in completion requests
- Python: `examples/json_schema_to_grammar.py`

### Supported JSON Schema Features
- `string`, `integer`, `number`, `boolean`, `array`, `object`
- `enum`, `const`, `pattern` (must start with `^`, end with `$`)
- `minLength`, `maxLength`, `minimum`, `maximum` (integers only)
- `required`, `properties`, `items`, `minItems`, `maxItems`
- `additionalProperties` (defaults to false)

### Limitations
- No `uniqueItems`, `contains`, `patternProperties`
- No remote `$ref` in C++ version
- No `if/then/else`, `not`, `$anchor`
- `prefixItems` broken (use `items`)
- No `uri`/`email` string formats

---

## GGUF Format

GGUF (GGML Universal File) is the model file format. It contains:
- Model metadata (architecture, tokenizer, hyperparameters)
- Tensor data (weights)
- Vocabulary
- Chat template

### Key Metadata Fields
| Field | Description |
|-------|-------------|
| `general.architecture` | Model architecture (llama, gemma, qwen, etc.) |
| `general.name` | Model name |
| `llama.context_length` | Training context size |
| `llama.embedding_length` | Embedding dimension |
| `llama.block_count` | Number of layers |
| `llama.attention.head_count` | Attention heads |
| `llama.attention.head_count_kv` | KV heads (for GQA) |
| `llama.rope.freq_base` | RoPE base frequency |
| `tokenizer.ggml.model` | Tokenizer type |
| `tokenizer.ggml.tokens` | Vocabulary |
| `tokenizer.ggml.add_bos_token` | BOS token flag |
| `tokenizer.ggml.add_eos_token` | EOS token flag |

---

## Key Data Structures

### llama_model_params
```c
struct llama_model_params {
    int32_t n_gpu_layers;
    enum llama_split_mode split_mode;
    int32_t main_gpu;
    const float * tensor_split;
    llama_progress_callback progress_callback;
    void * progress_callback_user_data;
    const struct llama_model_kv_override * kv_overrides;
    bool vocab_only;
    bool use_mmap;
    bool use_direct_io;
    bool use_mlock;
    bool check_tensors;
    bool use_extra_bufts;
    bool no_host;
    bool no_alloc;
};
```

### llama_context_params
```c
struct llama_context_params {
    uint32_t n_ctx;
    uint32_t n_batch;
    uint32_t n_ubatch;
    uint32_t n_seq_max;
    uint32_t n_rs_seq;
    uint32_t n_outputs_max;
    int32_t  n_threads;
    int32_t  n_threads_batch;
    enum llama_context_type ctx_type;
    enum llama_rope_scaling_type rope_scaling_type;
    enum llama_pooling_type pooling_type;
    enum llama_attention_type attention_type;
    enum llama_flash_attn_type flash_attn_type;
    float    rope_freq_base;
    float    rope_freq_scale;
    float    yarn_ext_factor;
    float    yarn_attn_factor;
    float    yarn_beta_fast;
    float    yarn_beta_slow;
    uint32_t yarn_orig_ctx;
    float    defrag_thold;
    enum ggml_type type_k;
    enum ggml_type type_v;
    ggml_abort_callback abort_callback;
    void * abort_callback_data;
    bool embeddings;
    bool offload_kqv;
    bool no_perf;
    bool op_offload;
    bool swa_full;
    bool kv_unified;
    struct llama_sampler_seq_config * samplers;
    size_t n_samplers;
    struct llama_context * ctx_other;
};
```

### llama_batch
```c
struct llama_batch {
    int32_t n_tokens;
    llama_token  * token;     // token IDs
    float        * embd;      // token embeddings (alternative to token)
    llama_pos    * pos;       // positions
    int32_t      * n_seq_id;  // number of sequence IDs per token
    llama_seq_id ** seq_id;   // sequence IDs
    int8_t       * logits;    // output flags (0 = no output)
};
```

### llama_sampler_i (VTable)
```c
struct llama_sampler_i {
    const char * (*name)(const struct llama_sampler * smpl);
    void (*accept)(struct llama_sampler * smpl, llama_token token);
    void (*apply)(struct llama_sampler * smpl, llama_token_data_array * cur_p);
    void (*reset)(struct llama_sampler * smpl);
    struct llama_sampler * (*clone)(const struct llama_sampler * smpl);
    void (*free)(struct llama_sampler * smpl);
    // Backend sampling (experimental)
    bool (*backend_init)(struct llama_sampler * smpl, ggml_backend_buffer_type_t buft);
    void (*backend_accept)(...);
    void (*backend_apply)(...);
    void (*backend_set_input)(...);
};
```

---

## Environment Variables

Most server/cli flags have corresponding `LLAMA_ARG_*` environment variables.

| Variable | Description |
|----------|-------------|
| `LLAMA_ARG_MODEL` | Model path |
| `LLAMA_ARG_CTX_SIZE` | Context size |
| `LLAMA_ARG_N_PREDICT` | Tokens to predict |
| `LLAMA_ARG_THREADS` | CPU threads |
| `LLAMA_ARG_N_GPU_LAYERS` | GPU layers |
| `LLAMA_ARG_N_PARALLEL` | Server slots |
| `LLAMA_ARG_PORT` | Server port |
| `LLAMA_ARG_HOST` | Server host |
| `LLAMA_ARG_BATCH` | Batch size |
| `LLAMA_ARG_UBATCH` | Physical batch size |
| `LLAMA_ARG_TEMPERATURE` | Temperature |
| `LLAMA_ARG_TOP_K` | Top-K |
| `LLAMA_ARG_TOP_P` | Top-P |
| `LLAMA_ARG_DEVICE` | Device list |
| `LLAMA_ARG_NUMA` | NUMA strategy |
| `LLAMA_API_KEY` | API key |
| `LLAMA_ARG_LOG_VERBOSITY` | Log verbosity (0-5) |
| `LLAMA_ARG_LOG_FILE` | Log file path |
| `LLAMA_ARG_FLASH_ATTN` | Flash attention (on/off/auto) |
| `LLAMA_ARG_CACHE_TYPE_K` | KV cache K type |
| `LLAMA_ARG_CACHE_TYPE_V` | KV cache V type |
| `LLAMA_ARG_SPLIT_MODE` | Multi-GPU split mode |
| `LLAMA_ARG_EMBEDDINGS` | Embedding mode |
| `LLAMA_ARG_RERANKING` | Reranking mode |
| `LLAMA_ARG_JINJA` | Jinja templates |
| `LLAMA_ARG_THINK` | Reasoning format |
| `LLAMA_ARG_REASONING` | Reasoning mode |
| `LLAMA_ARG_SPEC_TYPE` | Speculative decoding type |
| `LLAMA_ARG_HF_REPO` | Hugging Face repo |
| `LLAMA_ARG_HF_TOKEN` | Hugging Face token |
| `LLAMA_CACHE` | Model cache directory |
| `MODEL_ENDPOINT` | Custom model download endpoint |

### CUDA Environment Variables
| Variable | Description |
|----------|-------------|
| `CUDA_VISIBLE_DEVICES` | GPU device selection |
| `CUDA_SCALE_LAUNCH_QUEUES` | Command buffer size (e.g., `4x`) |
| `GGML_CUDA_ENABLE_UNIFIED_MEMORY` | Unified memory (Linux) |
| `GGML_CUDA_FORCE_MMQ` | Force MMQ kernels |
| `GGML_CUDA_FORCE_CUBLAS` | Force cuBLAS |
| `GGML_CUDA_FORCE_CUBLAS_COMPUTE_32F` | FP32 compute in cuBLAS |
| `GGML_CUDA_FORCE_CUBLAS_COMPUTE_16F` | FP16 compute in cuBLAS |
| `GGML_CUDA_P2P` | Peer-to-peer GPU access |
| `GGML_CUDA_PEER_MAX_BATCH_SIZE` | Max batch for P2P |
| `GGML_CUDA_FA_ALL_QUANTS` | All KV cache quant types for FA |

---

## Model Sources

### Hugging Face
```bash
llama-cli -hf ggml-org/gemma-3-1b-it-GGUF
llama-cli -hf ggml-org/gemma-3-1b-it-GGUF:Q4_K_M
llama-server -hf ggml-org/gemma-3-1b-it-GGUF
```

### Local Files
```bash
llama-cli -m /path/to/model.gguf
llama-cli -m /path/to/model-00001-of-00004.gguf  # Multi-shard
```

### Docker Hub
```bash
llama-cli -dr gemma3
```

### URL Download
```bash
llama-cli -mu https://example.com/model.gguf
```

### GGUF Shard Naming
Split files must follow: `<name>-%05d-of-%05d.gguf`
Use `llama_model_load_from_splits()` for custom naming.

---

## Multi-GPU & Backend Notes

### Split Modes
| Mode | Description |
|------|-------------|
| `none` | Single GPU |
| `layer` | Split layers across GPUs (pipelined) |
| `row` | Split weight rows (parallelized) |
| `tensor` | Split weights and KV (experimental) |

### Multi-Backend Builds
Multiple backends can be built simultaneously:
```bash
cmake -B build -DGGML_CUDA=ON -DGGML_VULKAN=ON
```

Use `--device` to select devices at runtime:
```bash
llama-cli -m model.gguf --device CUDA0,VULKAN0
```

Use `--list-devices` to see available devices.

### Dynamic Backend Loading
Build with `GGML_BACKEND_DL` to load backends as shared libraries at runtime.

### Performance Tips
- Use `--n-gpu-layers 99` (or `all`) to offload all layers to GPU
- Use `--device none` to fully disable GPU
- Enable Flash Attention with `-fa on`
- Use `--mlock` to keep model in RAM
- Use `--cache-type-k q8_0 --cache-type-v q8_0` for quantized KV cache
- Use `--cont-batching` for better throughput on server

---

## Quick Reference: Common Tasks

### Run a model interactively
```bash
llama-cli -m model.gguf -cnv -c 4096 -ngl 99
```

### Start an OpenAI-compatible server
```bash
llama-server -m model.gguf --port 8080 -c 4096 -ngl 99
```

### Quantize a model
```bash
python convert_hf_to_gguf.py --outfile model-f16.gguf --outtype bf16 --remote org/model
./build/bin/llama-quantize model-f16.gguf model-Q4_K_M.gguf Q4_K_M
```

### Benchmark
```bash
llama-bench -m model.gguf
```

### Use with cURL
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"model","messages":[{"role":"user","content":"Hello"}]}'
```

### Use with Python (openai library)
```python
from openai import OpenAI
client = OpenAI(base_url="http://localhost:8080/v1", api_key="sk-no-key-required")
response = client.chat.completions.create(
    model="model",
    messages=[{"role": "user", "content": "Hello"}]
)
print(response.choices[0].message.content)
```

---

*Last updated: 2026-06-29 from github.com/ggml-org/llama.cpp*
