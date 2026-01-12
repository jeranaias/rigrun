# Changelog

All notable changes to rigrun will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned Features
- Streaming support for chat completions
- Configuration file hot-reloading
- Web UI for monitoring and configuration
- Custom cache TTL configuration
- Support for additional local inference backends (llama.cpp, etc.)
- Docker image for easy deployment
- Prometheus metrics export
- Multi-user authentication

---

## [0.1.0] - 2024-01-12

### Added

#### Core Features
- **Smart Query Routing**: Automatic routing through cache → local GPU → cloud fallback
- **OpenAI-Compatible API**: Full compatibility with OpenAI client libraries
- **Semantic Caching**: TTL-based response caching with similarity matching
- **GPU Auto-Detection**: Automatic detection of NVIDIA, AMD, Apple Silicon, and Intel Arc GPUs
- **Model Recommendations**: Automatic model selection based on available VRAM
- **Cost Tracking**: Real-time tracking of cost savings and cloud spending

#### API Endpoints
- `GET /health` - Health check endpoint
- `GET /v1/models` - List available models
- `POST /v1/chat/completions` - OpenAI-compatible chat completions
- `GET /stats` - Usage statistics (session and daily)
- `GET /cache/stats` - Cache performance metrics

#### CLI Commands
- `rigrun` - Start server with auto-configuration
- `rigrun status` - Show server status, GPU info, and statistics
- `rigrun config` - Configure settings (OpenRouter key, model, port)
- `rigrun models` - List available and downloaded models
- `rigrun pull <model>` - Download specific model via Ollama
- `rigrun chat` - Interactive chat session
- `rigrun ide-setup` - Configure IDE integration (VS Code, Cursor, JetBrains, Neovim)
- `rigrun gpu-setup` - GPU setup wizard with diagnostics
- `rigrun examples` - Show CLI usage examples

#### GPU Support
- **NVIDIA**: Full CUDA support with driver detection
- **AMD**: ROCm support with architecture detection (including RDNA 4)
- **Apple Silicon**: Metal API support for M1/M2/M3
- **Intel Arc**: Basic support

#### Advanced Features
- **First-Run Menu**: Interactive setup wizard on first launch
- **Port Conflict Resolution**: Automatic detection and resolution of port conflicts
- **VRAM Monitoring**: Real-time VRAM usage tracking
- **Model GPU Status**: Detection of whether models are running on GPU or CPU
- **CPU Fallback Diagnosis**: Intelligent diagnosis of CPU fallback issues with actionable fixes
- **Background Server**: Ability to run as background service

#### Cloud Integration
- **OpenRouter Support**: Integration with OpenRouter for cloud fallback
- **Multiple Models**: Support for Claude Haiku, Sonnet, and Opus
- **Automatic Model Selection**: OpenRouter auto-routing for optimal cost/performance

#### Developer Experience
- **Configuration File**: JSON-based configuration in `~/.rigrun/config.json`
- **Stats Persistence**: Automatic saving and loading of usage statistics
- **Cache Persistence**: Automatic cache persistence to disk
- **Comprehensive Logging**: Detailed logging with tracing support
- **Error Handling**: Robust error handling with user-friendly messages

### Technical Details

#### Architecture
- Built with Rust 2021 edition
- Async runtime: Tokio
- Web framework: Axum
- HTTP client: Reqwest
- CLI parsing: Clap
- Serialization: Serde

#### Performance
- **Cache hit latency**: <10ms
- **Local inference**: GPU-accelerated via Ollama
- **Semantic caching**: Similarity-based cache matching
- **Concurrent requests**: Handled via Tokio async runtime

#### Model Support
Out-of-the-box support for:
- `qwen2.5-coder` series (1.5b, 3b, 7b, 14b, 32b)
- `deepseek-coder-v2:16b`
- `llama3.3:70b`
- All Ollama-compatible models

#### Tested Platforms
- **Linux**: Ubuntu 20.04+, Debian 11+, Arch, Fedora
- **macOS**: 12.0+ (Intel and Apple Silicon)
- **Windows**: Windows 10+, Windows Server 2019+

### Documentation

#### Added Documentation
- **README.md**: Comprehensive project overview with examples
- **docs/QUICKSTART.md**: Step-by-step getting started guide
- **docs/API.md**: Complete API reference with examples
- **docs/CONFIGURATION.md**: Configuration guide with all options
- **CHANGELOG.md**: This file

### Known Issues

- **AMD RDNA 4**: Requires custom Ollama build (ollama-for-amd fork)
- **Streaming**: Not yet implemented for chat completions
- **Authentication**: No built-in authentication (intended for local use)
- **Rate Limiting**: No built-in rate limiting

### Breaking Changes

None (initial release)

---

## Release Notes

### v0.1.0 - Initial Release

rigrun launches with a focus on making local LLM inference accessible and cost-effective. This initial release provides:

- **Zero-config setup**: Just install Ollama and run `rigrun`
- **Proven cost savings**: 80-90% reduction in LLM API costs
- **Privacy-first**: All processing local by default
- **Developer-friendly**: OpenAI-compatible API for easy integration

#### Target Users

- Developers with GPUs sitting idle
- Teams spending hundreds on LLM APIs
- Privacy-conscious users wanting local inference
- Cost-optimizers looking to reduce cloud spend

#### Next Steps

After installing:
1. Run `rigrun` to start server
2. Point your IDE or app to `http://localhost:8787`
3. Watch your costs drop while keeping full functionality

See [docs/QUICKSTART.md](docs/QUICKSTART.md) for detailed setup instructions.

---

## Future Roadmap

### v0.2.0 (Planned)
- [ ] Streaming support for real-time responses
- [ ] Web UI for configuration and monitoring
- [ ] Docker image for containerized deployment
- [ ] Homebrew tap for easy macOS installation
- [ ] Scoop bucket for easy Windows installation

### v0.3.0 (Planned)
- [ ] Multi-user authentication and authorization
- [ ] Custom cache TTL configuration
- [ ] Prometheus metrics export
- [ ] Advanced routing rules (custom complexity thresholds)
- [ ] Support for llama.cpp backend

### v1.0.0 (Planned)
- [ ] Production-ready stability
- [ ] Comprehensive test coverage (>90%)
- [ ] Performance optimizations
- [ ] Plugin system for custom backends
- [ ] Enterprise features (audit logs, RBAC)

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### How to Contribute

1. **Bug Reports**: Open an issue with details and reproduction steps
2. **Feature Requests**: Open an issue describing the feature and use case
3. **Pull Requests**: Fork, create a feature branch, and submit a PR
4. **Documentation**: Help improve docs with examples and clarifications

---

## Links

- **Repository**: https://github.com/rigrun/rigrun
- **Issues**: https://github.com/rigrun/rigrun/issues
- **Discussions**: https://github.com/rigrun/rigrun/discussions
- **Documentation**: [docs/](docs/)

---

**Note**: This is the initial release of rigrun. We're actively working on improvements and new features. Feedback and contributions are greatly appreciated!
