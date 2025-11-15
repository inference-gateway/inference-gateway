# Changelog

All notable changes to this project will be documented in this file.

## [0.19.6](https://github.com/inference-gateway/inference-gateway/compare/v0.19.5...v0.19.6) (2025-11-15)

### 🔧 Miscellaneous

* **docs:** Update Kubernetes examples to use k3s v1.34.1 and ingress-nginx v4.14.0 ([#191](https://github.com/inference-gateway/inference-gateway/issues/191)) ([65a1d53](https://github.com/inference-gateway/inference-gateway/commit/65a1d53e51e2d5d8fa0103de27412d79e14ad944))

## [0.19.5](https://github.com/inference-gateway/inference-gateway/compare/v0.19.4...v0.19.5) (2025-11-15)

### 🔧 Miscellaneous

* **deps:** Update cosign-installer to v4.0.0 in artifacts workflow ([#190](https://github.com/inference-gateway/inference-gateway/issues/190)) ([fae964c](https://github.com/inference-gateway/inference-gateway/commit/fae964c6b5a811d3523337f416160bb61552d796))

## [0.19.4](https://github.com/inference-gateway/inference-gateway/compare/v0.19.3...v0.19.4) (2025-11-15)

### 🔧 Miscellaneous

* **deps:** Update dependencies and delete claude code review workflow ([#186](https://github.com/inference-gateway/inference-gateway/issues/186)) ([184ed0d](https://github.com/inference-gateway/inference-gateway/commit/184ed0d91370365d51a0c322106e02c09c40ac3a))
* **deps:** Update quic-go dependency to v0.54.1 across all modules ([#189](https://github.com/inference-gateway/inference-gateway/issues/189)) ([58c157c](https://github.com/inference-gateway/inference-gateway/commit/58c157c46dde74a211e8f1ec5b0cbfb15751bbf4))

## [0.19.3](https://github.com/inference-gateway/inference-gateway/compare/v0.19.2...v0.19.3) (2025-09-29)

### ♻️ Improvements

* **a2a:** Remove A2A middleware and all related components ([#183](https://github.com/inference-gateway/inference-gateway/issues/183)) ([a32c7e4](https://github.com/inference-gateway/inference-gateway/commit/a32c7e4a08eae01868b645dc93521130db79de17))

### 🐛 Bug Fixes

* **a2a:** Prevent gateway crash when A2A agents fail to initialize ([#179](https://github.com/inference-gateway/inference-gateway/issues/179)) ([c44a211](https://github.com/inference-gateway/inference-gateway/commit/c44a211908c6078cbb1cfd41e983b6ff473c46bc)), closes [#178](https://github.com/inference-gateway/inference-gateway/issues/178)

### 🔧 Miscellaneous

* **flox:** Add claude-code package with version 1.0.65 ([594573a](https://github.com/inference-gateway/inference-gateway/commit/594573a5a1befe04098d5da9eb559873e9e454a6))

## [0.19.2](https://github.com/inference-gateway/inference-gateway/compare/v0.19.1...v0.19.2) (2025-09-02)

### 🐛 Bug Fixes

* **a2a:** Improve handleStreamingTaskSubmission to process text parts ([#180](https://github.com/inference-gateway/inference-gateway/issues/180)) ([a02db25](https://github.com/inference-gateway/inference-gateway/commit/a02db2597eae918fc60911d92bd9def02f84d4f4))

### 🔧 Miscellaneous

* **cli:** Set owner to 'inference-gateway' in config.yaml ([3b1d35e](https://github.com/inference-gateway/inference-gateway/commit/3b1d35e7d0dc636b424f0c794fec02ffec6bf4c0))
* **config:** Update protected paths to include all env files ([83d1c2a](https://github.com/inference-gateway/inference-gateway/commit/83d1c2a6faee455e9a89ac5a1d1ccab374b75b41))

## [0.19.2-rc.2](https://github.com/inference-gateway/inference-gateway/compare/v0.19.2-rc.1...v0.19.2-rc.2) (2025-08-29)

### 🐛 Bug Fixes

* **a2a:** Improve handleStreamingTaskSubmission to support both SSE and raw JSON formats for event parsing ([26dd6e5](https://github.com/inference-gateway/inference-gateway/commit/26dd6e5a65bcba1abb31678ca27958ffb68bd969))

## [0.19.2-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.19.1...v0.19.2-rc.1) (2025-08-29)

### 🐛 Bug Fixes

* **a2a:** Improve handleStreamingTaskSubmission to process text parts from task status updates ([2ea7276](https://github.com/inference-gateway/inference-gateway/commit/2ea72761f07cd95bf0de63f095b7bbc124ab592d))

### 🔧 Miscellaneous

* **cli:** Set owner to 'inference-gateway' in config.yaml ([3b1d35e](https://github.com/inference-gateway/inference-gateway/commit/3b1d35e7d0dc636b424f0c794fec02ffec6bf4c0))
* **config:** Update protected paths to include all env files ([83d1c2a](https://github.com/inference-gateway/inference-gateway/commit/83d1c2a6faee455e9a89ac5a1d1ccab374b75b41))

## [0.19.1](https://github.com/inference-gateway/inference-gateway/compare/v0.19.0...v0.19.1) (2025-08-23)

### 👷 CI

* Update golangci-lint installation script to version v2.4.0 in CI workflows ([5f00ebe](https://github.com/inference-gateway/inference-gateway/commit/5f00ebecfb2f04ab5fc4afd415e6a814a143408f))

### 📚 Documentation

* Add Google models to REST endpoints documentation ([658aaf9](https://github.com/inference-gateway/inference-gateway/commit/658aaf9734ce78a3dcf92644cf5c5c6433eee9cd))
* Improve README with CLI Tool section and installation instructions ([6d081a0](https://github.com/inference-gateway/inference-gateway/commit/6d081a0d66be82441962125b9e422426e4fa8aff))
* Update Google and Mistral links in metrics section ([f41c475](https://github.com/inference-gateway/inference-gateway/commit/f41c4759b47bc7afe7f0269e87e70d6f0ddf68fc))

### 🔧 Miscellaneous

* **cli:** Enable optimization and adjust parameters in config.yaml ([b5f3f59](https://github.com/inference-gateway/inference-gateway/commit/b5f3f597695ba6b5cb1931dc5c70dab709e27482))
* Update golangci-lint version and installation method in dev container ([8c4a4fd](https://github.com/inference-gateway/inference-gateway/commit/8c4a4fd635fd1a59cd084c2facfc4e442bd6a504))
* Update GoReleaser version to v2.11.2 in Dockerfile ([89ddec1](https://github.com/inference-gateway/inference-gateway/commit/89ddec167500923af977b88e96827553d4221d24))
* Update TASK_VERSION to v3.44.1 in dev container ([03eff9d](https://github.com/inference-gateway/inference-gateway/commit/03eff9d44126f41f63f24da1c2ca2f1aa3e5784f))

### 🔨 Miscellaneous

* Add Flox for development environment ([#175](https://github.com/inference-gateway/inference-gateway/issues/175)) ([2b306c3](https://github.com/inference-gateway/inference-gateway/commit/2b306c3b3c25f3cc9e62b3c71f880ab907f04032))
* Improve dev container caching and add Inference Gateway CLI ([d09691d](https://github.com/inference-gateway/inference-gateway/commit/d09691dd9a14fc7f842569d47637058290ba9b54))

## [0.19.0](https://github.com/inference-gateway/inference-gateway/compare/v0.18.0...v0.19.0) (2025-08-07)

### ✨ Features

* **providers:** Add Mistral AI as a provider ([#173](https://github.com/inference-gateway/inference-gateway/issues/173)) ([65d46dd](https://github.com/inference-gateway/inference-gateway/commit/65d46dd8971043dce320e21da7d3abe4c7062509))

## [0.18.0](https://github.com/inference-gateway/inference-gateway/compare/v0.17.2...v0.18.0) (2025-08-02)

### ✨ Features

* **debug:** Improve debug logging for better development experience ([#171](https://github.com/inference-gateway/inference-gateway/issues/171)) ([4bb1a47](https://github.com/inference-gateway/inference-gateway/commit/4bb1a47f056151c3d32e2fa632c2500a7b37d46f))

### 🔧 Miscellaneous

* Update inference-gateway image to version 0.17.2 ([a54adb4](https://github.com/inference-gateway/inference-gateway/commit/a54adb4295a352b7345df13db3ccc9b9caabe721))

## [0.17.2](https://github.com/inference-gateway/inference-gateway/compare/v0.17.1...v0.17.2) (2025-08-01)

### ♻️ Improvements

* **a2a:** Refactor service discovery to use Agent CRDs instead of A2A ([#169](https://github.com/inference-gateway/inference-gateway/issues/169)) ([d827be7](https://github.com/inference-gateway/inference-gateway/commit/d827be79afe39e10ee80a5c77276ced86b1286ed)), closes [#168](https://github.com/inference-gateway/inference-gateway/issues/168)

### 🔧 Miscellaneous

* Adjust the issues templates ([de3b255](https://github.com/inference-gateway/inference-gateway/commit/de3b25501c946b2ee595b316b453c68c8744c35d))

## [0.17.1](https://github.com/inference-gateway/inference-gateway/compare/v0.17.0...v0.17.1) (2025-07-30)

### ♻️ Improvements

* Update import paths from a2a to adk for consistency ([#167](https://github.com/inference-gateway/inference-gateway/issues/167)) ([99f57ed](https://github.com/inference-gateway/inference-gateway/commit/99f57ed6c66d627bc0bf01ce31747ea4a41030a9))

## [0.17.0](https://github.com/inference-gateway/inference-gateway/compare/v0.16.1...v0.17.0) (2025-07-29)

### ✨ Features

* **a2a:** Add Kubernetes service discovery for A2A agents ([#166](https://github.com/inference-gateway/inference-gateway/issues/166)) ([9be30ff](https://github.com/inference-gateway/inference-gateway/commit/9be30ff8c4a22fa26c4aef9bd00579e3cbb9b44a)), closes [#142](https://github.com/inference-gateway/inference-gateway/issues/142)

### 📚 Documentation

* Add a section to ensure this project remains independent and truly Open Source ([b970d8a](https://github.com/inference-gateway/inference-gateway/commit/b970d8ad47beb1a05aa40a0849bbeffb9e8e6ba4))

## [0.16.1](https://github.com/inference-gateway/inference-gateway/compare/v0.16.0...v0.16.1) (2025-07-26)

### ♻️ Improvements

* **workflow:** Remove security-events permission and scan_containers job ([ff0f482](https://github.com/inference-gateway/inference-gateway/commit/ff0f482feaf143ebfe8da036acdde9f948f3e5e2))

### 🔧 Miscellaneous

* **deps:** Add Dependabot configuration for gomod, docker, and GitHub Actions ([48d368a](https://github.com/inference-gateway/inference-gateway/commit/48d368a3292f97234cf34af72eee7f4aa2649ee0))
* **mcp:** Enhance MCP configuration with retry and health check options ([dedb7fd](https://github.com/inference-gateway/inference-gateway/commit/dedb7fd186045d08e62088dc66d585079fb801fb))

## [0.16.0](https://github.com/inference-gateway/inference-gateway/compare/v0.15.1...v0.16.0) (2025-07-26)

### ✨ Features

* **mcp:** Add health checks and retry mechanisms ([#165](https://github.com/inference-gateway/inference-gateway/issues/165)) ([f1ce6af](https://github.com/inference-gateway/inference-gateway/commit/f1ce6af42a04cb6cc450702873862db17e509b3d)), closes [#164](https://github.com/inference-gateway/inference-gateway/issues/164)

### 👷 CI

* Change permissions for Claude Code Review workflow to read-only for contents ([88a9822](https://github.com/inference-gateway/inference-gateway/commit/88a9822206343c8f91c19520f28e0f4723b54dfe))

## [0.15.1](https://github.com/inference-gateway/inference-gateway/compare/v0.15.0...v0.15.1) (2025-07-26)

### ♻️ Improvements

* Download the latest A2A schema and generate new Go types ([#163](https://github.com/inference-gateway/inference-gateway/issues/163)) ([3c88346](https://github.com/inference-gateway/inference-gateway/commit/3c88346610b625f0858babf5e30bbde533aea7a6))
* Run mcp:schema:download to fetch the latest schema changes and update the code ([#162](https://github.com/inference-gateway/inference-gateway/issues/162)) ([7082068](https://github.com/inference-gateway/inference-gateway/commit/70820683371f7a91860143a116da8d62e2856f41))

### 📚 Documentation

* Add Google to the list of supported providers ([9bc07ba](https://github.com/inference-gateway/inference-gateway/commit/9bc07ba9d0b6ffdef149c82b7682d0a18ebe1787))
* Improve TOC ([13161e2](https://github.com/inference-gateway/inference-gateway/commit/13161e277c4f8880850dc6a360d3a298c67892d7))

### 🔧 Miscellaneous

* Add .prettierrc for consistent single quote style ([8c7dd57](https://github.com/inference-gateway/inference-gateway/commit/8c7dd57eddd1cc3330df36b13a66d990d0a74a8e))
* Update OpenAPI related styling ([be2ce52](https://github.com/inference-gateway/inference-gateway/commit/be2ce52061f8c6649c5ded65d46441257e39cbdb))
* Update task commands for MCP and A2A schema downloads to use new format ([f3ebcb9](https://github.com/inference-gateway/inference-gateway/commit/f3ebcb995de0d04392ae089128a6b4db5261a527))
* Update YAML example in README for consistent single quote style ([d61b18b](https://github.com/inference-gateway/inference-gateway/commit/d61b18be95934e9fa29816547588c219e4b00d68))

### 🎨 Miscellaneous

* Add code formatting step to pre-commit checks ([17964be](https://github.com/inference-gateway/inference-gateway/commit/17964be3daab6c1f9e071353beb7586b90a27cad))

## [0.15.0](https://github.com/inference-gateway/inference-gateway/compare/v0.14.1...v0.15.0) (2025-07-26)

### ✨ Features

* **providers:** Add Google OpenAI-compatible API provider ([#161](https://github.com/inference-gateway/inference-gateway/issues/161)) ([d367fe9](https://github.com/inference-gateway/inference-gateway/commit/d367fe924091291a62013bbe5e1cd3d83a4a6082)), closes [#146](https://github.com/inference-gateway/inference-gateway/issues/146)

### ♻️ Improvements

* **auth:** Rename authentication configuration variables to use AUTH_ prefix ([b960377](https://github.com/inference-gateway/inference-gateway/commit/b9603775bed5f478db783a73638260daba07c46d))

### 🔨 Miscellaneous

* **pre-commit:** Add pre-commit hooks for code quality checks ([#160](https://github.com/inference-gateway/inference-gateway/issues/160)) ([bf011af](https://github.com/inference-gateway/inference-gateway/commit/bf011af2eeec602c1c0a7c0f05fb20ee2480215b))

## [0.14.1](https://github.com/inference-gateway/inference-gateway/compare/v0.14.0...v0.14.1) (2025-07-25)

### ♻️ Improvements

* **config:** Refactor authentication config to use AUTH_ prefix ([#159](https://github.com/inference-gateway/inference-gateway/issues/159)) ([c97bdd1](https://github.com/inference-gateway/inference-gateway/commit/c97bdd15a14f067a5d46eb501dabc22e3f816cb6))
* **docs:** Remove outdated section on function/tool call metrics from README ([9ec242c](https://github.com/inference-gateway/inference-gateway/commit/9ec242c4a26c016a1a694654729375bfec74c5bc))

### 🔧 Miscellaneous

* Update inference-gateway version to 0.14.0 in Taskfiles and README ([ea4c25e](https://github.com/inference-gateway/inference-gateway/commit/ea4c25e7a382ac0f0d1d2410dfb569ea7bafadef))

## [0.14.0](https://github.com/inference-gateway/inference-gateway/compare/v0.13.0...v0.14.0) (2025-07-25)

### ✨ Features

* **config:** Add configurable TELEMETRY_METRICS_PORT setting ([#152](https://github.com/inference-gateway/inference-gateway/issues/152)) ([daa066e](https://github.com/inference-gateway/inference-gateway/commit/daa066e08e45c5e17a61319e0fcf3724ddf79259)), closes [#151](https://github.com/inference-gateway/inference-gateway/issues/151)
* **mcp:** Add "mcp_" prefix to MCP tools for better classification ([#157](https://github.com/inference-gateway/inference-gateway/issues/157)) ([1bbab6d](https://github.com/inference-gateway/inference-gateway/commit/1bbab6d310005532cb2744eb8b61f54c281d47ff)), closes [#155](https://github.com/inference-gateway/inference-gateway/issues/155)
* **metrics:** Add more function/tool call metrics ([#148](https://github.com/inference-gateway/inference-gateway/issues/148)) ([98ea636](https://github.com/inference-gateway/inference-gateway/commit/98ea6369c96f982e60fd9c8c18cdb0b1dc946dd3))

### ♻️ Improvements

* **A2A:** Refactor tool names to use consistent a2a_ prefix ([#150](https://github.com/inference-gateway/inference-gateway/issues/150)) ([bb0738e](https://github.com/inference-gateway/inference-gateway/commit/bb0738e13c2b90d41f57445a16392b925b006b23)), closes [#147](https://github.com/inference-gateway/inference-gateway/issues/147)
* **config:** Rename ENABLE_TELEMETRY to TELEMETRY_ENABLE ([#154](https://github.com/inference-gateway/inference-gateway/issues/154)) ([92c27f8](https://github.com/inference-gateway/inference-gateway/commit/92c27f898372199df5d3651eca32dc0dea99f535)), closes [#153](https://github.com/inference-gateway/inference-gateway/issues/153)

### 📚 Documentation

* Update README to clarify middleware toggling and enhance flow diagram ([#145](https://github.com/inference-gateway/inference-gateway/issues/145)) ([9479086](https://github.com/inference-gateway/inference-gateway/commit/947908654913f55eb871f0d07365f03bf934547f))

## [0.13.0](https://github.com/inference-gateway/inference-gateway/compare/v0.12.0...v0.13.0) (2025-07-25)

### ✨ Features

* **a2a:** Implement retry mechanism for agent connections ([#140](https://github.com/inference-gateway/inference-gateway/issues/140)) ([54033e8](https://github.com/inference-gateway/inference-gateway/commit/54033e8ef4a5489bb6212b715e7f34a7e1d3931a)), closes [#139](https://github.com/inference-gateway/inference-gateway/issues/139)
* Implement A2A agent status polling with background health checks ([#136](https://github.com/inference-gateway/inference-gateway/issues/136)) ([1b49a06](https://github.com/inference-gateway/inference-gateway/commit/1b49a06bc10f54f7d91cefe1a6643643c9e5d402)), closes [#135](https://github.com/inference-gateway/inference-gateway/issues/135)

### ♻️ Improvements

* **codegen:** Refactor code generation to automate provider onboarding ([#144](https://github.com/inference-gateway/inference-gateway/issues/144)) ([3a97396](https://github.com/inference-gateway/inference-gateway/commit/3a9739672ab8da1702cbfcaadebe1082b1b39d1f))
* Replace custom A2A code with ADK client implementation ([#138](https://github.com/inference-gateway/inference-gateway/issues/138)) ([34d8cf6](https://github.com/inference-gateway/inference-gateway/commit/34d8cf633446b30bdbb2cf232b9ac10e5c6c08e3))

### 👷 CI

* Add Claude GitHub Actions workflows ([#134](https://github.com/inference-gateway/inference-gateway/issues/134)) ([a6a1f8f](https://github.com/inference-gateway/inference-gateway/commit/a6a1f8fa3a51b9c6470b49cd3e83545ebab74b37))
* Add MCP configuration for context7 in Claude workflows ([4ce0139](https://github.com/inference-gateway/inference-gateway/commit/4ce01391b957d9e20e282effff9e3f6c7fb705ed))
* **fix:** Add allowed tools configuration for Bash tasks in Claude workflow ([ccf76c8](https://github.com/inference-gateway/inference-gateway/commit/ccf76c88752fe4c9e21f0c5a26bdf42ed6fe9e70))
* **fix:** Add base branch and branch prefix configuration with custom instructions for workflow ([8d3a56e](https://github.com/inference-gateway/inference-gateway/commit/8d3a56e09ddf6ea2e97af50654d7113d79fac0c5))
* **fix:** Add installation steps for golangci-lint and task in Claude workflow ([e2a718f](https://github.com/inference-gateway/inference-gateway/commit/e2a718f23bb7eb91be099c25370b2f0aaa9d17d6))
* **fix:** Reduce amounts of claude runs and costs - update workflow trigger to respond to issue comments for code review ([189313b](https://github.com/inference-gateway/inference-gateway/commit/189313bf93dd41c24aabe9cd053258ef6cb4a62c))
* **fix:** Update Claude workflow conditions to exclude review commands from triggering ([5e3d75d](https://github.com/inference-gateway/inference-gateway/commit/5e3d75d8908ccb7184c43c52374bfc92dc6df09a))
* Update Claude workflows to require write permissions for contents, pull requests, and issues ([ba6477e](https://github.com/inference-gateway/inference-gateway/commit/ba6477e71785eb71c557911d87a9e2d60f675a54))

### 📚 Documentation

* **examples:** Update kubernetes examples to use the inference gateway operator ([#131](https://github.com/inference-gateway/inference-gateway/issues/131)) ([3ab617a](https://github.com/inference-gateway/inference-gateway/commit/3ab617abd75414fb25a9bcff438f98c637de005c))

## [0.12.0](https://github.com/inference-gateway/inference-gateway/compare/v0.11.2...v0.12.0) (2025-06-18)

### ✨ Features

* A2A - Add ListAgentsHandler and GetAgentHandler endpoint to retrieve specific agent details by ID ([#129](https://github.com/inference-gateway/inference-gateway/issues/129)) ([2250fba](https://github.com/inference-gateway/inference-gateway/commit/2250fbabb15746a21a508806f73d5941d7588091))

### 📚 Documentation

* Update README with new A2A integration examples for Docker and Kubernetes ([#128](https://github.com/inference-gateway/inference-gateway/issues/128)) ([cd7a12a](https://github.com/inference-gateway/inference-gateway/commit/cd7a12a0c8f111a53b0bdbf47094e947cdb9716e))

## [0.11.2](https://github.com/inference-gateway/inference-gateway/compare/v0.11.1...v0.11.2) (2025-06-15)

### ♻️ Improvements

* Move all MCP related logic to the Middleware ([#127](https://github.com/inference-gateway/inference-gateway/issues/127)) ([6e4375e](https://github.com/inference-gateway/inference-gateway/commit/6e4375eb0e890ce770958023b4fca0cf3e7294e2))

## [0.11.1](https://github.com/inference-gateway/inference-gateway/compare/v0.11.0...v0.11.1) (2025-06-15)

### ♻️ Improvements

* Rename internal headers to use 'Bypass' terminology ([#126](https://github.com/inference-gateway/inference-gateway/issues/126)) ([c93c75c](https://github.com/inference-gateway/inference-gateway/commit/c93c75c975565e0190f0ff0ee00763a31a582dd7))

## [0.11.0](https://github.com/inference-gateway/inference-gateway/compare/v0.10.2...v0.11.0) (2025-06-12)

### ✨ Features

* **a2a:** Implement A2A streaming mode and response handling ([#124](https://github.com/inference-gateway/inference-gateway/issues/124)) ([269c74f](https://github.com/inference-gateway/inference-gateway/commit/269c74f38ceeb8bbab89c516df2a7810a29d534d))

## [0.10.2](https://github.com/inference-gateway/inference-gateway/compare/v0.10.1...v0.10.2) (2025-06-10)

### ♻️ Improvements

* **docker:** Add healthchecker service and healthcheck script for A2A agents ([2ae49e5](https://github.com/inference-gateway/inference-gateway/commit/2ae49e5e03e3f8acb30c8dab4d33416fc9026cdb))
* Use internal shared tools repository for code generation ([#125](https://github.com/inference-gateway/inference-gateway/issues/125)) ([65dc0e5](https://github.com/inference-gateway/inference-gateway/commit/65dc0e52fe6436d829b36658b3e4643f06cbcaa3))

### 📚 Documentation

* **examples:** Update example environment files and docker-compose configuration for Google Calendar integration ([d1cdc4d](https://github.com/inference-gateway/inference-gateway/commit/d1cdc4dd82b51a9f64b3f955af55116958c1ed65))
* Improve Copilot custom instructions - remove duplicated and copy custom instructions to a CLAUDE.md should someone want to use Claude code ([8c4b66a](https://github.com/inference-gateway/inference-gateway/commit/8c4b66ae3e5feb50565ccae60246d9613c4c3065))

## [0.10.1](https://github.com/inference-gateway/inference-gateway/compare/v0.10.0...v0.10.1) (2025-06-08)

### 🔧 Miscellaneous

* Update dependencies and improve Taskfile commands ([#116](https://github.com/inference-gateway/inference-gateway/issues/116)) ([886ba61](https://github.com/inference-gateway/inference-gateway/commit/886ba6198cd63e42326d7c86a55dc02343249ecb))
* Upgrade Go version to 1.24 across all Dockerfiles and go.mod files ([e019b18](https://github.com/inference-gateway/inference-gateway/commit/e019b189e89d6a1d8ce03266ea07e6eb056b8791))

### ✅ Miscellaneous

* **fix:** Update model IDs in benchmark tests to include namespace ([c71c127](https://github.com/inference-gateway/inference-gateway/commit/c71c127d9f17a04094c6420c3945d318c9f724a6))

## [0.10.0](https://github.com/inference-gateway/inference-gateway/compare/v0.9.0...v0.10.0) (2025-06-08)

### ✨ Features

* Implement Google's A2A Client ([#110](https://github.com/inference-gateway/inference-gateway/issues/110)) ([bae9711](https://github.com/inference-gateway/inference-gateway/commit/bae9711d03a683884717d886659363158aeedd1e)), closes [#115](https://github.com/inference-gateway/inference-gateway/issues/115)

### 📚 Documentation

* Update MCP server endpoints to use streamableHttp servers ([9959238](https://github.com/inference-gateway/inference-gateway/commit/9959238401e84800e77fc3a523e375e4ec6409cc))

## [0.10.0-rc.2](https://github.com/inference-gateway/inference-gateway/compare/v0.10.0-rc.1...v0.10.0-rc.2) (2025-06-08)

### 🐛 Bug Fixes

* Rename 'type' to 'kind' in sendMessageWithTextPart ([69c8255](https://github.com/inference-gateway/inference-gateway/commit/69c82555a8e36bcf65d5e3b0150f7f4f93beb916))

## [0.10.0-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.9.0...v0.10.0-rc.1) (2025-06-08)

### ✨ Features

* **a2a:** Implement A2A agents listing and client initialization ([c16cff4](https://github.com/inference-gateway/inference-gateway/commit/c16cff49ffb0c5cba12ecb4246b0e34fe6754c6b))
* **a2a:** Implement progressive discovery of agent skills and enhance tool handling ([998a871](https://github.com/inference-gateway/inference-gateway/commit/998a8719b90db151b6893c0210260f405e77532f))
* Add A2A configuration support in the code generation ([2b35dd8](https://github.com/inference-gateway/inference-gateway/commit/2b35dd8fad121acbc1a23ea5b39d1e30066580b1))
* Add A2A configuration support in the Config struct ([a1a3301](https://github.com/inference-gateway/inference-gateway/commit/a1a3301ada40cbd19dc09f82cc7a088d631eb9bb))
* Add A2A directory to Dockerfile and implement build container task in Taskfile ([687da83](https://github.com/inference-gateway/inference-gateway/commit/687da8300f94f00426b77d666390659706eab866))
* Add A2A expose option to components for agent list cards endpoint ([0ca8dcb](https://github.com/inference-gateway/inference-gateway/commit/0ca8dcb4ba8a64f901e8f9e4a84b8f932312fc1d))
* Add A2A JSON Schema Definitions ([f0c72a4](https://github.com/inference-gateway/inference-gateway/commit/f0c72a4bb6cc24fe8ead1dccde73516d1161c7f3))
* Add A2A schema download task and update generation commands ([09a4adb](https://github.com/inference-gateway/inference-gateway/commit/09a4adb3a853e4372fef5b8cf4d78faf60f2a1c5))
* Add A2A types generation from schema in the generate command ([01a7989](https://github.com/inference-gateway/inference-gateway/commit/01a79893adf80ccaa02a26b42a5ed4e099b03d5e))
* Add Agent-to-Agent (A2A) protocol support with configuration options ([3c746df](https://github.com/inference-gateway/inference-gateway/commit/3c746df44c8d873a0a24494853eb4ec59f72b10e))
* Add CalendarService interface implementation check and clean up message stream handling ([7d8265f](https://github.com/inference-gateway/inference-gateway/commit/7d8265f6854b78092378473443d6864f3e48a2d3))
* Add support for Spanish greetings in message handling ([1e2dba9](https://github.com/inference-gateway/inference-gateway/commit/1e2dba9f35d58ec8d4c2e9ec2383a1ddf079a3f3))
* Enhance A2A agent functionality and improve message handling ([0e14b3d](https://github.com/inference-gateway/inference-gateway/commit/0e14b3df8deea90fc0fce58e40d70888f843ecfd))
* Generate - add A2A_EXPOSE configuration for agent list cards endpoint ([699f2c4](https://github.com/inference-gateway/inference-gateway/commit/699f2c4ad3d84aef16ac9f4de4340f29dffa4e84))
* Implement A2A middleware and client ([c8a9b2c](https://github.com/inference-gateway/inference-gateway/commit/c8a9b2c158930b68d20bf3b6608b3698816a9f46))
* Implement GenerateA2ATypes function for A2A JSON/YAML schema processing ([58d8025](https://github.com/inference-gateway/inference-gateway/commit/58d80258587ed062e65fea035d0bae165f164d5e))
* Integrate structured logging with zap and enhance greeting generation logic ([aa9e7fe](https://github.com/inference-gateway/inference-gateway/commit/aa9e7fef03d25e1f50335172e7b8b5f03e7cc04e))
* **middleware:** Add agent skill tools to A2A middleware and enhance logging ([20f9738](https://github.com/inference-gateway/inference-gateway/commit/20f97388e5f90ecb7827d7e0d8eaa98b8062b760))
* Run generate - add Agent-to-Agent (A2A) protocol configuration options ([7ddd9c2](https://github.com/inference-gateway/inference-gateway/commit/7ddd9c28e0c2ef0eff4762266628367d5c030359))
* **schema:** Download the latest schema from a2a official repository - Add extensions property to AgentCapabilities and related definitions ([a45a5e8](https://github.com/inference-gateway/inference-gateway/commit/a45a5e825c81adbc10387e43beb24f9a5ac2de72))
* **schema:** Run generate - add extensions property to AgentCapabilities and new AgentExtension type ([c48fcd4](https://github.com/inference-gateway/inference-gateway/commit/c48fcd41a68eb9f464db78d7b15dea121c482ee3))
* Update Google Calendar agent configuration and logging enhancements ([b5108c6](https://github.com/inference-gateway/inference-gateway/commit/b5108c6f3148d547250270f97961c52045e5dab0))

### ♻️ Improvements

* Ensure defending errors checks ([89f2851](https://github.com/inference-gateway/inference-gateway/commit/89f2851ba7178aab64a2f9a0c4460f17047dff2d))
* Refactor tests to use generated mocks for providers ([abd9b40](https://github.com/inference-gateway/inference-gateway/commit/abd9b40581d52a2e8030425ba7b48a5c02bd4df4))
* Simplify logging in Google Calendar service and enhance calendar access checks ([602245f](https://github.com/inference-gateway/inference-gateway/commit/602245f680f79ff572a6dd782c915e10a7677111))
* **tests:** Create a separation of concern between mcp and a2a testing mocks ([e6513ad](https://github.com/inference-gateway/inference-gateway/commit/e6513ad85226cef86f9ba61161cbc6a18ddcc994))
* **tests:** Remove redundant comments in A2A middleware tests ([1ab8869](https://github.com/inference-gateway/inference-gateway/commit/1ab8869993cafcf97507548f130f7556c7cd6a9c))

### 🐛 Bug Fixes

* **a2a:** Add ALLOWED_MODELS variable to .env.example and mock ListAgentsHandler in routes ([c1170d3](https://github.com/inference-gateway/inference-gateway/commit/c1170d38969f8e008d95f9c9a3e6d73cbb3257ad))
* **a2a:** Update agent card URL to use the correct path for agent.json ([8b7e5f1](https://github.com/inference-gateway/inference-gateway/commit/8b7e5f18d35a29568da0cd72c8d3dd7e6a199fc8))
* **client:** Update JSON-RPC URL path in makeJSONRPCRequest method ([0a2bace](https://github.com/inference-gateway/inference-gateway/commit/0a2bace880ea557d4c0a37791472050fd723ba04))
* **docs:** Update model name in README for, groq deprecated the other one ([6dc0505](https://github.com/inference-gateway/inference-gateway/commit/6dc05053e751ad72b425499a0f02665479e59930))
* **docs:** Update README to replace Groq API key with DeepSeek API key and adjust model references ([1885793](https://github.com/inference-gateway/inference-gateway/commit/1885793762f2eb7d1ed2dab328fd74de56c3f9c5))
* **middleware:** Handle missing provider in A2A middleware and log error ([afd785b](https://github.com/inference-gateway/inference-gateway/commit/afd785b9436622980fb57c215bbb764d42279e0e))
* Remove indirect requirement for github.com/google/uuid ([2467b85](https://github.com/inference-gateway/inference-gateway/commit/2467b8542d9b61a0969207607e3c91fce3c5a350))
* Update Dockerfile and main.go for Google Calendar agent configuration and service scopes ([c960f7d](https://github.com/inference-gateway/inference-gateway/commit/c960f7d0bc0011193cd9788cfd92599342b00e77))

### 📚 Documentation

* Add A2A integration section to README with usage instructions ([b91f6cb](https://github.com/inference-gateway/inference-gateway/commit/b91f6cb06b61f2f78c002b0273947374303c6aed))
* Add README for A2A protocol integration and usage instructions ([dc7638d](https://github.com/inference-gateway/inference-gateway/commit/dc7638d2fcb5265b5f8a3a7131b143d5a40793db))
* **examples-a2a:** Add HelloWorld and Weather agents with A2A protocol support ([b2f7a64](https://github.com/inference-gateway/inference-gateway/commit/b2f7a64edd3be07c23a759cfb36b6e317c349059))
* **examples:** Add unit tests for A2A Google Calendar agent ([db5186d](https://github.com/inference-gateway/inference-gateway/commit/db5186df48793cb91c60231598ffc3babf55ef67))
* **examples:** Vendor the a2a package for now ([66b4da1](https://github.com/inference-gateway/inference-gateway/commit/66b4da1d4e462fb616432f4e297410b93831c1d3))
* **fix:** Update A2A agent endpoints in README to reflect correct ports ([113b434](https://github.com/inference-gateway/inference-gateway/commit/113b434120fbdbce42f6146ca4e35f464d9bb456))
* Update MCP server endpoints to use streamableHttp servers ([9959238](https://github.com/inference-gateway/inference-gateway/commit/9959238401e84800e77fc3a523e375e4ec6409cc))

### 🔧 Miscellaneous

* Add guideline for using lowercase log messages for consistency ([e010cc2](https://github.com/inference-gateway/inference-gateway/commit/e010cc23d1389a053eeb1c33843d11bf1c541448))
* Add TODOs ([4528828](https://github.com/inference-gateway/inference-gateway/commit/4528828ceee4dc493dbf0bb8dfd09d5bde24222e))
* Fast-forward - merge branch 'main' into feature/implement-googles-a2a-client ([f055cfd](https://github.com/inference-gateway/inference-gateway/commit/f055cfda0598f00c6b69d92c5c7400e6a9514db3))
* Mark a2a/generated_types.go as generated in .gitattributes ([7276b29](https://github.com/inference-gateway/inference-gateway/commit/7276b29938836a83e8fb25ced74e79a70089c069))
* Move agent to a dedicated project ([59a7eca](https://github.com/inference-gateway/inference-gateway/commit/59a7eca187ea3ca63cea27ee52504f88d4092462))

### ✅ Miscellaneous

* **a2a:** Add ListAgentsHandler to expose A2A agents endpoint with detailed response handling ([f7364b6](https://github.com/inference-gateway/inference-gateway/commit/f7364b6030c888b0a9cfa179df3ae1cc748eebfa))
* Add A2A configuration options to TestLoad function ([ddb854a](https://github.com/inference-gateway/inference-gateway/commit/ddb854a0b833d67e4d396ee0bd41c54899b912fa))
* Add mock implementation for A2AClientInterface using MockGen ([2dde66e](https://github.com/inference-gateway/inference-gateway/commit/2dde66e0808f7670f9bd0c7bb4afa027a33335f9))

## [0.9.0](https://github.com/inference-gateway/inference-gateway/compare/v0.8.1...v0.9.0) (2025-06-02)

### ✨ Features

* Add ALLOWED_MODELS configuration for model filtering ([#108](https://github.com/inference-gateway/inference-gateway/issues/108)) ([9d3dcfe](https://github.com/inference-gateway/inference-gateway/commit/9d3dcfe50eb7c608895e5d4cf6b9344a46d515c7))

### ♻️ Improvements

* Remove redundant debug logs in RunWithStream and update log messages in MCPMiddleware ([#112](https://github.com/inference-gateway/inference-gateway/issues/112)) ([ca9659e](https://github.com/inference-gateway/inference-gateway/commit/ca9659ea5b5f858440dccc4efd648dbb595ffe53))

### 📚 Documentation

* **typo:** Correct NoOpLogger to NoopLogger in comments ([48c4065](https://github.com/inference-gateway/inference-gateway/commit/48c4065de7358fdf60e4ce0249513fef3bc49dfe))
* Update model identifiers to include OpenAI namespace in examples ([ecf3d23](https://github.com/inference-gateway/inference-gateway/commit/ecf3d23e42b6ac97cbe3bb4b2f7b398ce1059a28))

## [0.8.1](https://github.com/inference-gateway/inference-gateway/compare/v0.8.0...v0.8.1) (2025-05-31)

### ♻️ Improvements

* Standardize Structured Logging ([#107](https://github.com/inference-gateway/inference-gateway/issues/107)) ([d0dfcc9](https://github.com/inference-gateway/inference-gateway/commit/d0dfcc9d09de947dc61f251bd29aa0806915364b))

## [0.8.0](https://github.com/inference-gateway/inference-gateway/compare/v0.7.6...v0.8.0) (2025-05-31)

### ✨ Features

* Add client configuration options for compression and timeouts ([#105](https://github.com/inference-gateway/inference-gateway/issues/105)) ([6f32205](https://github.com/inference-gateway/inference-gateway/commit/6f322053030485fba9a860f217561bcc342c171d)), closes [#106](https://github.com/inference-gateway/inference-gateway/issues/106)

## [0.8.0-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.7.6...v0.8.0-rc.1) (2025-05-30)

### ✨ Features

* Add client configuration options for compression and timeouts ([c56d681](https://github.com/inference-gateway/inference-gateway/commit/c56d681dbb643a2d6b73caf90a47666f469fdb3b))

### ♻️ Improvements

* Enhance streaming support and optimize request handling in provider ([a5fee60](https://github.com/inference-gateway/inference-gateway/commit/a5fee603783e9c61469a0c5952817985ccc3c51c))

### 🐛 Bug Fixes

* Improve logging for streaming requests and responses, enhance development environment ([7cc37ef](https://github.com/inference-gateway/inference-gateway/commit/7cc37ef1126b916c1c59748999a15366529e2155))
* Update inference-gateway service to use pre-built image instead of build context ([f1396cd](https://github.com/inference-gateway/inference-gateway/commit/f1396cd2805f0ca6dd0abd49d3fb60ecbf6be518))

## [0.7.6](https://github.com/inference-gateway/inference-gateway/compare/v0.7.5...v0.7.6) (2025-05-29)

### 🐛 Bug Fixes

* Enhance provider determination logic and update error messages ([#104](https://github.com/inference-gateway/inference-gateway/issues/104)) ([535ca09](https://github.com/inference-gateway/inference-gateway/commit/535ca09be3d8cb4e71673681f6f1119d5153a004))

### 🔧 Miscellaneous

* Fix typo ([6f7a882](https://github.com/inference-gateway/inference-gateway/commit/6f7a882c3624c6b61196d8167792f1689009864e))

## [0.7.5](https://github.com/inference-gateway/inference-gateway/compare/v0.7.4...v0.7.5) (2025-05-29)

### ♻️ Improvements

* **mcp:** Extract MCP middleware functionality to a dedicated agent ([#103](https://github.com/inference-gateway/inference-gateway/issues/103)) ([b7be164](https://github.com/inference-gateway/inference-gateway/commit/b7be16414bb55dff04604afa71e5831cabc89b6d))

## [0.7.4](https://github.com/inference-gateway/inference-gateway/compare/v0.7.3...v0.7.4) (2025-05-28)

### 📚 Documentation

* Fix formatting in README for MCP Integration section ([e4ca17e](https://github.com/inference-gateway/inference-gateway/commit/e4ca17e6223bf4c0c39b92f362e43e5bbd06bc4d))
* **kubernetes-mcp-example:** Split MCP servers into their own directories with multiple files so it's easier to understand what's going on ([#88](https://github.com/inference-gateway/inference-gateway/issues/88)) ([1b5f469](https://github.com/inference-gateway/inference-gateway/commit/1b5f469badc121895da6d9aefdf1d4d9b798ed01))

### 🔧 Miscellaneous

* Add GitHub command configuration to MCP settings ([18ea534](https://github.com/inference-gateway/inference-gateway/commit/18ea534f1d928ae5889c6c197b54b88376df7685))
* **examples:** Update .gitignore for filesystem-data directory ([2117066](https://github.com/inference-gateway/inference-gateway/commit/2117066bec5ffca2615a54d557536066a0d8ba8b))
* Update .gitattributes to include examples directory as vendored ([4fd9a07](https://github.com/inference-gateway/inference-gateway/commit/4fd9a07bd3cfc25d3cded09644cc84600e001db6))

### 🔨 Miscellaneous

* **deps:** bump golang.org/x/net ([#89](https://github.com/inference-gateway/inference-gateway/issues/89)) ([809634a](https://github.com/inference-gateway/inference-gateway/commit/809634acba74d9e3dea39ace112e3e842c9387aa))

### 🔒 Security

* Address a potential Identity Token leakage to MCP servers ([22db3c0](https://github.com/inference-gateway/inference-gateway/commit/22db3c0358fa0db24998c0a7178b0a620bbf2f6b))
* **deps:** Bump github.com/gin-gonic/gin ([#102](https://github.com/inference-gateway/inference-gateway/issues/102)) ([cb49859](https://github.com/inference-gateway/inference-gateway/commit/cb4985951b2b2eda8c5ac0d597eeb5cf60a76747))
* **deps:** Bump github.com/gin-gonic/gin ([#91](https://github.com/inference-gateway/inference-gateway/issues/91)) ([45933cf](https://github.com/inference-gateway/inference-gateway/commit/45933cfe6e3af7ce6d50ff10b6d6e860f1cd71e0))
* **deps:** Bump github.com/gin-gonic/gin ([#95](https://github.com/inference-gateway/inference-gateway/issues/95)) ([8289ffd](https://github.com/inference-gateway/inference-gateway/commit/8289ffdb5810417a1d2a84fcfa06d6f96246501a))
* **deps:** Bump github.com/gin-gonic/gin ([#98](https://github.com/inference-gateway/inference-gateway/issues/98)) ([b25e563](https://github.com/inference-gateway/inference-gateway/commit/b25e5634e5d9dedc7a5a8f8dca69b8c736f7cc63))
* **deps:** Bump golang.org/x/net ([#100](https://github.com/inference-gateway/inference-gateway/issues/100)) ([cfbebd0](https://github.com/inference-gateway/inference-gateway/commit/cfbebd0997a1a71292a9e217fd45728065ec5bdb))
* **deps:** Bump golang.org/x/net ([#92](https://github.com/inference-gateway/inference-gateway/issues/92)) ([ae0439e](https://github.com/inference-gateway/inference-gateway/commit/ae0439eac7239a6d5160748dfb315069316f6a8a))
* **deps:** Bump golang.org/x/net ([#93](https://github.com/inference-gateway/inference-gateway/issues/93)) ([fa92f31](https://github.com/inference-gateway/inference-gateway/commit/fa92f3163f343adde762ac7a5dd9ef8e551daa0c))
* **deps:** Bump golang.org/x/net ([#96](https://github.com/inference-gateway/inference-gateway/issues/96)) ([43d84d4](https://github.com/inference-gateway/inference-gateway/commit/43d84d48bbc1fba41cef8146e112a391813107d2))
* **deps:** Bump golang.org/x/net ([#99](https://github.com/inference-gateway/inference-gateway/issues/99)) ([3281ffd](https://github.com/inference-gateway/inference-gateway/commit/3281ffd716b21d9e05f6b2a95d96a42234580b9a))
* **deps:** Bump google.golang.org/protobuf ([#101](https://github.com/inference-gateway/inference-gateway/issues/101)) ([0e311df](https://github.com/inference-gateway/inference-gateway/commit/0e311dfae62629a9816f29fac6dc947c706fbe63))
* **deps:** Bump google.golang.org/protobuf ([#90](https://github.com/inference-gateway/inference-gateway/issues/90)) ([37808d6](https://github.com/inference-gateway/inference-gateway/commit/37808d6bdc39d886edb90ca12cc29d26bbb67a4f))
* **deps:** Bump google.golang.org/protobuf ([#94](https://github.com/inference-gateway/inference-gateway/issues/94)) ([521e869](https://github.com/inference-gateway/inference-gateway/commit/521e869aba73a28e5fb065b80915fe8dd035d1c6))
* **deps:** Bump google.golang.org/protobuf ([#97](https://github.com/inference-gateway/inference-gateway/issues/97)) ([d521a7c](https://github.com/inference-gateway/inference-gateway/commit/d521a7c0c16307299ab784df6d2d8dc59cab12b3))
* **docs:** Update dependencies for crypto, sys, and text packages to latest versions ([ae7b0b9](https://github.com/inference-gateway/inference-gateway/commit/ae7b0b9d2aea3337f29ce94e1a5673d6a5182318))

## [0.7.3](https://github.com/inference-gateway/inference-gateway/compare/v0.7.2...v0.7.3) (2025-05-28)

### ♻️ Improvements

* MCP Handling it should be compatible with the official MCP Typescript SDK ([#87](https://github.com/inference-gateway/inference-gateway/issues/87)) ([4e97133](https://github.com/inference-gateway/inference-gateway/commit/4e971332ce3c7a00873a001ae4202b6d6ce7a716))

### 🐛 Bug Fixes

* Allow additional properties in MCPTool input schema ([8a4739c](https://github.com/inference-gateway/inference-gateway/commit/8a4739c3c4bba91744f02f5f84ece0407b5f409b))

### 🔧 Miscellaneous

* Add node_modules to .gitignore globally for the examples ([7b2f58c](https://github.com/inference-gateway/inference-gateway/commit/7b2f58cde3f42408469d44ccd632b37e9378ff03))

## [0.7.2](https://github.com/inference-gateway/inference-gateway/compare/v0.7.1...v0.7.2) (2025-05-27)

### 🐛 Bug Fixes

* When MCP is Enabled and there are no configured servers or the configured server is unavailable return an empty slice ([#82](https://github.com/inference-gateway/inference-gateway/issues/82)) ([36c122b](https://github.com/inference-gateway/inference-gateway/commit/36c122b4d1e6a813490f08e0ccbc293a5c867c32))

### 📚 Documentation

* **examples:** Update examples to use the latest pre built OCIs ([#80](https://github.com/inference-gateway/inference-gateway/issues/80)) ([92896af](https://github.com/inference-gateway/inference-gateway/commit/92896affea5ae1ae621fed522be2b727d38bd9e0))

### 🔧 Miscellaneous

* **cleanup:** Update copilot-instructions.md ([4089a13](https://github.com/inference-gateway/inference-gateway/commit/4089a13ed1ba02d54e023c3c8c2019969c4fa146))

## [0.7.1](https://github.com/inference-gateway/inference-gateway/compare/v0.7.0...v0.7.1) (2025-05-26)

### 🐛 Bug Fixes

* MCP client initialization and better warning feedback ([#79](https://github.com/inference-gateway/inference-gateway/issues/79)) ([cf9868b](https://github.com/inference-gateway/inference-gateway/commit/cf9868bde08d3563bffbfc5189c9dd9df9d4256d))

## [0.7.0](https://github.com/inference-gateway/inference-gateway/compare/v0.6.3...v0.7.0) (2025-05-25)

### ✨ Features

* Implement Comprehensive Model Context Protocol (MCP) Integration ([#72](https://github.com/inference-gateway/inference-gateway/issues/72)) ([0cf5752](https://github.com/inference-gateway/inference-gateway/commit/0cf5752416e9fc5d0c36602af362383fae6d4289)), closes [#78](https://github.com/inference-gateway/inference-gateway/issues/78)

### 📚 Documentation

* Improve docs - mention the UI project ([#77](https://github.com/inference-gateway/inference-gateway/issues/77)) ([be5cb70](https://github.com/inference-gateway/inference-gateway/commit/be5cb702b1265e44d41185c117f364758a93ac38))
* Update the Examples ([#76](https://github.com/inference-gateway/inference-gateway/issues/76)) ([a35b7ac](https://github.com/inference-gateway/inference-gateway/commit/a35b7ac06efb45c1d10d5767e90dd46d32e1d070))

## [0.7.0-rc.2](https://github.com/inference-gateway/inference-gateway/compare/v0.7.0-rc.1...v0.7.0-rc.2) (2025-05-25)

### 🐛 Bug Fixes

* **mcp:** Add timeout handling for MCP client initialization and log success message ([5c44d65](https://github.com/inference-gateway/inference-gateway/commit/5c44d6524d6982449ffd0737a21fdce28125e960))
* **mcp:** Increase timeout values and update log commands for MCP servers ([30a3b4d](https://github.com/inference-gateway/inference-gateway/commit/30a3b4df644cf1ddd408e1c339711c871032226a))

### 📚 Documentation

* Add note for skipping setup step in VSCode dev container ([4946dcb](https://github.com/inference-gateway/inference-gateway/commit/4946dcb44487999775be93fe5005ba48663777e3))
* **fix:** Update README to clarify optional API key setup and ensure MCP servers are deployed ([207bef7](https://github.com/inference-gateway/inference-gateway/commit/207bef7f3dfeb97ca07e2a3dd74d3169651fe08e))

### 🔧 Miscellaneous

* update CHART_VERSION to 0.7.0-rc.1 in Taskfile.yaml ([c527c58](https://github.com/inference-gateway/inference-gateway/commit/c527c58ed7939f72ced2359bcba7da6154886d64))

## [0.7.0-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.6.3...v0.7.0-rc.1) (2025-05-25)

### ✨ Features

* Add MCP client timeout configurations and update related components ([2ac15c5](https://github.com/inference-gateway/inference-gateway/commit/2ac15c5384dec2276e05126edc43e426efd402dd))
* Add MCP middleware tests and mock implementations ([022f654](https://github.com/inference-gateway/inference-gateway/commit/022f654e2fe284fbeb24076cf73bda8ab043df4f))
* Add MCP tools endpoint and expose configuration ([024b29c](https://github.com/inference-gateway/inference-gateway/commit/024b29c7c6bbd885395de21be7d53964292c47c8))
* Enhance MCP middleware error handling and response formatting ([1cadb80](https://github.com/inference-gateway/inference-gateway/commit/1cadb80857ea7f639d4276113a8ca78cde073cde))
* Enhance MCP streaming response handling and add examples to documentation ([0c348ad](https://github.com/inference-gateway/inference-gateway/commit/0c348ad2c7d7fbed7b7f2e7e4880a76104a898f0))
* Implement MCP client interface and middleware integration with tool execution capabilities ([333f8e7](https://github.com/inference-gateway/inference-gateway/commit/333f8e714875967ce1d72acf210e199797e199f3))
* Implement the standard HTTP MCP middleware and enhance MCP Time Server in the examples ([e3a07e6](https://github.com/inference-gateway/inference-gateway/commit/e3a07e6ee06f46191215d64aa537a7a6873ce590))
* Integrate Model Context Protocol (MCP) support with middleware and configuration options ([e6a1f04](https://github.com/inference-gateway/inference-gateway/commit/e6a1f040e3089239ea7fdad5305e106848325263))

### ♻️ Improvements

* Create a dedicated Model Context Protocol (MCP) configuration section and documentation updates ([53da3dd](https://github.com/inference-gateway/inference-gateway/commit/53da3dd89e19628ec464a7fce9bc4a45d53a8ae2))
* **mcp:** Simplify MCPClientInterface - only add the methods needed ([3f95f75](https://github.com/inference-gateway/inference-gateway/commit/3f95f757ed4ba452ab05b9c155528dce4f7409ff))
* **middleware:** Enhance MCP middleware to process tool calls only when absolutely necessary ([daa53a7](https://github.com/inference-gateway/inference-gateway/commit/daa53a78210eae92cebca2533cfc5f8bc3d9a6ac))
* **middleware:** Implement chat completions endpoint and enhance request/response handling, use pre-defined types. ([a897d54](https://github.com/inference-gateway/inference-gateway/commit/a897d54258ec81af6fe8ffa0cfb3740108644c86))
* **mocks:** Run task generate - Remove unused DiscoverCapabilities and StreamChatWithTools methods from MockMCPClientInterface ([3c55bd3](https://github.com/inference-gateway/inference-gateway/commit/3c55bd34f0a898e47be48314cf79c1af89b812e0))
* Refactor MCP Middleware and Client Implementation ([c19a8e5](https://github.com/inference-gateway/inference-gateway/commit/c19a8e5e70a042b36d84759f000ed6c3489ccd49))
* Refactor MCP middleware tests and update mock implementations ([2f0b886](https://github.com/inference-gateway/inference-gateway/commit/2f0b8861e43933121dea22980d53ba230e29ef81))
* Remove unnecessary else statement ([f3947ec](https://github.com/inference-gateway/inference-gateway/commit/f3947ec065b2c9fb786ae9c34327a7e5dc91ae4b))
* Sync latest changes from Anthropic official MCP implementation ([59e5ba4](https://github.com/inference-gateway/inference-gateway/commit/59e5ba4233b122b7217d032d52e5470782faf40f))
* Update MCP types generation and improve schema format handling ([ccfadfd](https://github.com/inference-gateway/inference-gateway/commit/ccfadfd7362314fc40e5ede14df4649e7d79e879))

### 🐛 Bug Fixes

* Add error handling for response and request body encoding/decoding in MCP middleware and tests ([8ed9901](https://github.com/inference-gateway/inference-gateway/commit/8ed9901ef14ec949964616c53fac64976ae81a40))
* Correct gitattributes entry for mcp/generated_types.go ([4479724](https://github.com/inference-gateway/inference-gateway/commit/4479724564e3be703e57ebd995714ab2d66b0982))
* **docker:** Ensure MCP directory is copied in Dockerfile for proper build ([2977b4f](https://github.com/inference-gateway/inference-gateway/commit/2977b4f2d6f8adcb09c7b13930bb25d742e7a96e))
* **middleware:** Add streaming request handling and context key for SSE events ([a664bb4](https://github.com/inference-gateway/inference-gateway/commit/a664bb4a53c59bf49acc161b5c6b2f74aea5314d))
* **test:** Update expected response structure in MCP middleware tests to reflect changes in content handling instead of in response ([37bc074](https://github.com/inference-gateway/inference-gateway/commit/37bc0747690cfd0caa61ad9efd1dde0615b79d3c))
* Update environment variables in docker-compose for MCP configuration ([ce7060d](https://github.com/inference-gateway/inference-gateway/commit/ce7060d6be6d97d2bf06b51c473d4f5bc0f5ec72))

### 📚 Documentation

* Add MCP support to README and update architecture diagram ([1892bde](https://github.com/inference-gateway/inference-gateway/commit/1892bdeb51016c47e5d07462aa7d560618e48fef))
* Add MCP_EXPOSE environment variable to README for exposing MCP endpoints ([5bada81](https://github.com/inference-gateway/inference-gateway/commit/5bada81846cb23aa0cdd1c2587a2379246a358fa))
* **examples-mcp-filesystem:** Add filesystem server to MCP with file management capabilities ([d37cfed](https://github.com/inference-gateway/inference-gateway/commit/d37cfeda61cc6fd188bf1d39237e126351c26e47))
* **examples-mcp-kubernetes:** Add MCP Time Server deployment, service, and configuration ([731575e](https://github.com/inference-gateway/inference-gateway/commit/731575e65909e644a473bf4b652487f0ce1ddcf5))
* **examples:** Add MCP Search Server and update README with search functionality ([b4be79a](https://github.com/inference-gateway/inference-gateway/commit/b4be79ad8016b250eab4464072a9d2a0f87fcbb3))
* **fix:** Rename environment variable for enabling MCP middleware ([cb4d932](https://github.com/inference-gateway/inference-gateway/commit/cb4d932d4823a183b55a7f8e05798e1b770fa5e5))
* Improve docs - mention the UI project ([#77](https://github.com/inference-gateway/inference-gateway/issues/77)) ([be5cb70](https://github.com/inference-gateway/inference-gateway/commit/be5cb702b1265e44d41185c117f364758a93ac38))
* Update README with comprehensive Table of Contents and MCP Inspector details ([55eab5e](https://github.com/inference-gateway/inference-gateway/commit/55eab5e547c9cd55a2dc50bebd1f3e546aa9659d))
* Update the Examples ([#76](https://github.com/inference-gateway/inference-gateway/issues/76)) ([a35b7ac](https://github.com/inference-gateway/inference-gateway/commit/a35b7ac06efb45c1d10d5767e90dd46d32e1d070))

### 🔧 Miscellaneous

* Add MCP client timeout configurations and update related documentation ([e275623](https://github.com/inference-gateway/inference-gateway/commit/e275623835cd168c711b51440a396b4b6f7e3061))
* Enhance MCP middleware to process tool calls and handle response body ([d22a871](https://github.com/inference-gateway/inference-gateway/commit/d22a871b6e65f11f85ceec8277c1aff0286c0b83))
* Merge branch 'main' into feature/implement-mcp-middleware ([afd1372](https://github.com/inference-gateway/inference-gateway/commit/afd13722f14f004145e7c0a61c9bf746b5a3dc78))
* Remove unused indirect dependencies from go.mod and go.sum ([65cf0a2](https://github.com/inference-gateway/inference-gateway/commit/65cf0a23471136a1cc318adca5cd3e0b85007701))
* Update go.uber.org/mock to v0.5.2 and add new indirect dependencies ([75e29e2](https://github.com/inference-gateway/inference-gateway/commit/75e29e29f083d9f665babf7d571a42b6cbbd905d))

### 🔨 Miscellaneous

* Add MCP configuration for fetch command in devcontainer ([3c08b93](https://github.com/inference-gateway/inference-gateway/commit/3c08b9376934a94c08806faf29645c7bb5a09c2e))
* Add MCP Context7 to fetch latest documentation about any library ([268e9ee](https://github.com/inference-gateway/inference-gateway/commit/268e9eed4b3db9c03b258b00be7df1afc4e867c2))
* Improve codegen - add MCP types and update client interfaces ([399a5d5](https://github.com/inference-gateway/inference-gateway/commit/399a5d554b3c6db01dff7b2cfb107ea5665c0c76))

## [0.6.3](https://github.com/inference-gateway/inference-gateway/compare/v0.6.2...v0.6.3) (2025-05-22)

### 🐛 Bug Fixes

* Enhance error handling in ChatCompletions and ListModels methods ([#75](https://github.com/inference-gateway/inference-gateway/issues/75)) ([0c57534](https://github.com/inference-gateway/inference-gateway/commit/0c5753477fefa003970fa36f585bc53ffd618a72))

## [0.6.2](https://github.com/inference-gateway/inference-gateway/compare/v0.6.1...v0.6.2) (2025-05-21)

### ♻️ Improvements

* Improve development experience and CD ([#73](https://github.com/inference-gateway/inference-gateway/issues/73)) ([74e28d9](https://github.com/inference-gateway/inference-gateway/commit/74e28d9d4592e53c10594372b402ea03e9db4ac9)), closes [#74](https://github.com/inference-gateway/inference-gateway/issues/74)

### 📚 Documentation

* **examples:** Update model reference in README and ensure port mapping in docker-compose ([dd93a75](https://github.com/inference-gateway/inference-gateway/commit/dd93a755b1317e08bfb3e739c09a63389ad4df28))

## [0.6.2-rc.2](https://github.com/inference-gateway/inference-gateway/compare/v0.6.2-rc.1...v0.6.2-rc.2) (2025-05-21)

### 🐛 Bug Fixes

* Enhance Docker setup with multi-architecture support and manifest inspection ([2d8b04e](https://github.com/inference-gateway/inference-gateway/commit/2d8b04e1b33521659a87258165297b516b418696))

## [0.6.2-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.6.1...v0.6.2-rc.1) (2025-05-21)

### ♻️ Improvements

* Update Docker Buildx setup and streamline GoReleaser configuration for multi-architecture support ([3585c8a](https://github.com/inference-gateway/inference-gateway/commit/3585c8a087faa069b629a1f158b695dc362f60f4))

### 📚 Documentation

* **examples:** Update model reference in README and ensure port mapping in docker-compose ([dd93a75](https://github.com/inference-gateway/inference-gateway/commit/dd93a755b1317e08bfb3e739c09a63389ad4df28))

### 🔧 Miscellaneous

* Refactor devcontainer setup by removing unnecessary files and updating zsh configuration ([eb07cd5](https://github.com/inference-gateway/inference-gateway/commit/eb07cd541e15e6b48e62476cab700057c9c84418))
* Update Copilot instructions to emphasize linting and testing before commits ([66c3a16](https://github.com/inference-gateway/inference-gateway/commit/66c3a16936a5e5b6e57f8dd8965fc0d925cd8e9c))

## [0.6.1](https://github.com/inference-gateway/inference-gateway/compare/v0.6.0...v0.6.1) (2025-04-30)

### 🐛 Bug Fixes

* Add reasoning also to delta stream messages ([5c4a172](https://github.com/inference-gateway/inference-gateway/commit/5c4a1721df0636b6534f42dce08e0931d7fe567c))

## [0.6.0](https://github.com/inference-gateway/inference-gateway/compare/v0.5.7...v0.6.0) (2025-04-29)

### ✨ Features

* Add reasoning_format attribute to support Groq reasoning models better response format ([#71](https://github.com/inference-gateway/inference-gateway/issues/71)) ([feea4d6](https://github.com/inference-gateway/inference-gateway/commit/feea4d622b8adb0509b0e5c7991d477102d482c9))

## [0.5.7](https://github.com/inference-gateway/inference-gateway/compare/v0.5.6...v0.5.7) (2025-04-27)

### 🔨 Miscellaneous

* Streamline OpenAPI property Function Tools parameters and generate the types ([#70](https://github.com/inference-gateway/inference-gateway/issues/70)) ([1ff9a12](https://github.com/inference-gateway/inference-gateway/commit/1ff9a1284604f4a82f5c62d779deef6bbffc1663))

## [0.5.6](https://github.com/inference-gateway/inference-gateway/compare/v0.5.5...v0.5.6) (2025-04-15)

### 🔒 Security

* **fix:** Update golang.org/x/net to v0.39.0 to fix CVE-2023-45288 ([#68](https://github.com/inference-gateway/inference-gateway/issues/68)) ([ff90bed](https://github.com/inference-gateway/inference-gateway/commit/ff90bed3e6c385c1a5041e0c8c6f7c61c5bfd3e5))

## [0.5.5](https://github.com/inference-gateway/inference-gateway/compare/v0.5.4...v0.5.5) (2025-04-15)

### 📚 Documentation

* Add Tools example to docker-compose ([#63](https://github.com/inference-gateway/inference-gateway/issues/63)) ([75bb135](https://github.com/inference-gateway/inference-gateway/commit/75bb135f8c795dfdcbe810c698419374b16e6149))
* Add UI deployment example and Taskfile for Inference Gateway ([#64](https://github.com/inference-gateway/inference-gateway/issues/64)) ([1ce4d27](https://github.com/inference-gateway/inference-gateway/commit/1ce4d27a08688f4bed5afa66a45e16475129bf10))
* **examples:** Docker compose examples ([#62](https://github.com/inference-gateway/inference-gateway/issues/62)) ([164e867](https://github.com/inference-gateway/inference-gateway/commit/164e8672cf55c7ba9b5e5ea2d0b07153225de790))
* **fix:** Update Inference Gateway UI helm chart version to 0.5.0 ([98b4396](https://github.com/inference-gateway/inference-gateway/commit/98b4396994f830e8028624c603ed6b60f26ed7b9))

### 🔒 Security

* **deps:** Bump golang.org/x/crypto from 0.32.0 to 0.35.0 ([#65](https://github.com/inference-gateway/inference-gateway/issues/65)) ([92debd7](https://github.com/inference-gateway/inference-gateway/commit/92debd70799542ca25f36487ef95a9e42bf858f5))
* **deps:** bump golang.org/x/net from 0.34.0 to 0.36.0 ([#66](https://github.com/inference-gateway/inference-gateway/issues/66)) ([d5da4db](https://github.com/inference-gateway/inference-gateway/commit/d5da4db9d12a4b991e5ae30d1dee84d3de1876e6))

## [0.5.4](https://github.com/inference-gateway/inference-gateway/compare/v0.5.3...v0.5.4) (2025-04-14)

### 🐛 Bug Fixes

* Ensure ListModelsResponse fields are required and initialize allModels if nil ([#61](https://github.com/inference-gateway/inference-gateway/issues/61)) ([0a0211a](https://github.com/inference-gateway/inference-gateway/commit/0a0211afa9b1aae6856aa6922d87f9f642c7faa4))

## [0.5.3](https://github.com/inference-gateway/inference-gateway/compare/v0.5.2...v0.5.3) (2025-04-12)

### 🐛 Bug Fixes

* Update Helm chart references to version 0.5.0 in examples Taskfiles ([09c899e](https://github.com/inference-gateway/inference-gateway/commit/09c899ec3faf50cff1e52d0a26d582ae92b614ca))

### 📚 Documentation

* **fix:** Update Keycloak access URL in README and add Bitnami repo to Taskfile ([22597e4](https://github.com/inference-gateway/inference-gateway/commit/22597e4f9d6b8e18981be5c4a9299427c835a9c4))

## [0.5.2](https://github.com/inference-gateway/inference-gateway/compare/v0.5.1...v0.5.2) (2025-04-12)

### 🐛 Bug Fixes

* Try something - update Helm chart publishing to use latest version for non-rc tags ([f53540d](https://github.com/inference-gateway/inference-gateway/commit/f53540d6cc4972adf5b69b43143edfa336a9f228))

## [0.5.1](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0...v0.5.1) (2025-04-12)

### 🐛 Bug Fixes

* Add conditional versioning for Helm chart publishing ([543b95b](https://github.com/inference-gateway/inference-gateway/commit/543b95b5788b215cb23d8cf14f2e34d5c73d6e47))

## [0.5.0](https://github.com/inference-gateway/inference-gateway/compare/v0.4.1...v0.5.0) (2025-04-12)

### ✨ Features

* Add inference-gateway Helm chart and monitoring support ([#59](https://github.com/inference-gateway/inference-gateway/issues/59)) ([5178355](https://github.com/inference-gateway/inference-gateway/commit/51783554832425df14826a188d44f39fd00bcc05)), closes [#60](https://github.com/inference-gateway/inference-gateway/issues/60)

## [0.5.0-rc.24](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.23...v0.5.0-rc.24) (2025-04-11)

### 🐛 Bug Fixes

* Add labels to GitHub release configuration ([f4fe902](https://github.com/inference-gateway/inference-gateway/commit/f4fe902e0d791f80642ed5db8441b765bc2eddbe))

## [0.5.0-rc.23](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.22...v0.5.0-rc.23) (2025-04-11)

### 🐛 Bug Fixes

* Add TAG environment variable to sign container images job ([c82b6c8](https://github.com/inference-gateway/inference-gateway/commit/c82b6c85f430bd814986c132540b2f57057a9465))

## [0.5.0-rc.22](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.21...v0.5.0-rc.22) (2025-04-11)

### 🐛 Bug Fixes

* Signing container digest - pull the image and sign it ([aacccb9](https://github.com/inference-gateway/inference-gateway/commit/aacccb9d638b1b051ce032a40485eeeb0defa1fc))

## [0.5.0-rc.21](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.20...v0.5.0-rc.21) (2025-04-11)

### 🐛 Bug Fixes

* Remove unnecessary image tag patterns in container signing step ([5c06cdd](https://github.com/inference-gateway/inference-gateway/commit/5c06cdde60ea271d929ea718ebcbd5e0069467d4))

## [0.5.0-rc.20](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.19...v0.5.0-rc.20) (2025-04-11)

### 🐛 Bug Fixes

* Update image reference in vulnerability scanning to use env.VERSION ([2eb446f](https://github.com/inference-gateway/inference-gateway/commit/2eb446f68515bbb6c699d2889fe022523b208124))

### 🔧 Miscellaneous

* Update inference-gateway chart reference to version 0.5.0-rc.19 ([1ee2b69](https://github.com/inference-gateway/inference-gateway/commit/1ee2b694bed29c16c3db80eabb4016c185b07ec4))

## [0.5.0-rc.19](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.18...v0.5.0-rc.19) (2025-04-11)

### 🐛 Bug Fixes

* Update vulnerability scanning image reference to use VERSION environment variable ([7b3d70d](https://github.com/inference-gateway/inference-gateway/commit/7b3d70d413a2b45bb0872111fb7db8278aea4692))

## [0.5.0-rc.18](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.17...v0.5.0-rc.18) (2025-04-11)

### 🐛 Bug Fixes

* Update environment variable usage for container image tagging in artifacts.yml ([35764d3](https://github.com/inference-gateway/inference-gateway/commit/35764d30b05c38eded533d9332624b6c5b731a14))
* Update image reference format for vulnerability scanning in artifacts.yml ([2b2572c](https://github.com/inference-gateway/inference-gateway/commit/2b2572c8983fd497ab5d8067568f67ac7d0d53dc))

## [0.5.0-rc.17](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.16...v0.5.0-rc.17) (2025-04-11)

### 🐛 Bug Fixes

* Update environment variable for container image tagging in artifacts.yml ([1614673](https://github.com/inference-gateway/inference-gateway/commit/1614673df67694119a95ee1ab9f06683dbd671d9))

## [0.5.0-rc.16](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.15...v0.5.0-rc.16) (2025-04-11)

### ♻️ Improvements

* Update Docker image tag to use version instead of tag ([1854d7f](https://github.com/inference-gateway/inference-gateway/commit/1854d7f0c8c1710bbd3b717856d1c0f34c8b8ae2))

## [0.5.0-rc.15](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.14...v0.5.0-rc.15) (2025-04-11)

### ♻️ Improvements

* Remove redundant if check ([17eba04](https://github.com/inference-gateway/inference-gateway/commit/17eba04bf56361b00098dd78071841344d8f9f83))
* Update Helm chart image tag and add environment variable for development ([d3c5e65](https://github.com/inference-gateway/inference-gateway/commit/d3c5e65088d51fb72043040e214be32c6b501c3e))

### 🐛 Bug Fixes

* Correct image tag formatting in deployment.yaml ([98b687c](https://github.com/inference-gateway/inference-gateway/commit/98b687ce90481cb5b336e393f58aeebe235a9e5d))
* Update image tag in Docker configuration to use version instead of tag ([7950c24](https://github.com/inference-gateway/inference-gateway/commit/7950c24152c11fda665ec74e754be6a686294716))

### 🔧 Miscellaneous

* Move repositoryUrl and tagFormat to the correct position in .releaserc.yaml ([dc97d0f](https://github.com/inference-gateway/inference-gateway/commit/dc97d0f1e8fb1130f42fc033371b7d5ee29783f7))

## [0.5.0-rc.14](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.13...v0.5.0-rc.14) (2025-04-10)

### 🐛 Bug Fixes

* Restore id-token permission in GitHub Actions workflow - need it for signing the container images ([6b9507f](https://github.com/inference-gateway/inference-gateway/commit/6b9507f598c44c1b5fc017e52edfd6e50e511b04))
* Update Helm chart push destination to use repository owner instead of repository ([39e47a6](https://github.com/inference-gateway/inference-gateway/commit/39e47a69234bd603bfa34f8e024fcc9dbf4728b1))

## [0.5.0-rc.13](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.12...v0.5.0-rc.13) (2025-04-10)

### ♻️ Improvements

* Cleanup - Simplify GitHub Actions workflow by removing GitHub App token steps and using GITHUB_TOKEN ([23d7cf6](https://github.com/inference-gateway/inference-gateway/commit/23d7cf689b86113b7cd48d2e00bf4e60ff9a73f9))

### 🐛 Bug Fixes

* Update permissions to allow write access for contents in GitHub Actions workflow to allow it to upload security scans ([9ffd72c](https://github.com/inference-gateway/inference-gateway/commit/9ffd72cca49c60350705ea6196b2cc5884adbc2d))

## [0.5.0-rc.12](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.11...v0.5.0-rc.12) (2025-04-10)

### 🐛 Bug Fixes

* Try with the standard GITHUB access token ([fc3e555](https://github.com/inference-gateway/inference-gateway/commit/fc3e55578b0fcd9afecd1c6fc5eceff2b64f876f))
* Update appVersion in Chart.yaml during release preparation ([84f6ac6](https://github.com/inference-gateway/inference-gateway/commit/84f6ac6997b223cab7773dd604ab8e6b99ea56aa))

## [0.5.0-rc.11](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.10...v0.5.0-rc.11) (2025-04-10)

### 🔒 Security

* Comment out unnecessary permissions in artifacts workflow ([c34db50](https://github.com/inference-gateway/inference-gateway/commit/c34db5008e53f0572347029b99c27251a72783d2))

## [0.5.0-rc.10](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.9...v0.5.0-rc.10) (2025-04-10)

### 🔒 Security

* Remove unnecessary permissions from upload_artifacts job ([e3d0eb0](https://github.com/inference-gateway/inference-gateway/commit/e3d0eb07fec14117ec50ce3a8b1d91daa44230c0))

## [0.5.0-rc.9](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.8...v0.5.0-rc.9) (2025-04-10)

### ♻️ Improvements

* Debug if it's a local issue or configuration issue ([5e87d94](https://github.com/inference-gateway/inference-gateway/commit/5e87d945221c860b0ef45c135d38e76403c6cdd0))

## [0.5.0-rc.8](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.7...v0.5.0-rc.8) (2025-04-10)

### ♻️ Improvements

* Try something ([3fa7dfb](https://github.com/inference-gateway/inference-gateway/commit/3fa7dfbefd69d0e108b5538b387909f3d3e7048d))

## [0.5.0-rc.7](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.6...v0.5.0-rc.7) (2025-04-10)

### 🐛 Bug Fixes

* Remove redundant permissions from sign_containers job in artifacts workflow ([ecff024](https://github.com/inference-gateway/inference-gateway/commit/ecff024e5eebecb55ec95479489688d8c7530708))
* Remove unnecessary environment variables and options from GitHub Actions jobs ([a16f4a7](https://github.com/inference-gateway/inference-gateway/commit/a16f4a7f6667c29dd9f819412cd21e22f53d80b5))

## [0.5.0-rc.6](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.5...v0.5.0-rc.6) (2025-04-10)

### 🐛 Bug Fixes

* Add permissions for contents and packages in artifacts workflow ([9540ea3](https://github.com/inference-gateway/inference-gateway/commit/9540ea31ff239d34cf452fd2132703f63d860af0))

## [0.5.0-rc.5](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.4...v0.5.0-rc.5) (2025-04-10)

### 👷 CI

* **release:** Integrate GitHub App token management for artifact uploads and container scans ([30a5a0d](https://github.com/inference-gateway/inference-gateway/commit/30a5a0d38ae77034e418417d48cfd00edcd3d8de))

## [0.5.0-rc.4](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.3...v0.5.0-rc.4) (2025-04-10)

### 🐛 Bug Fixes

* Refactor Helm chart packaging and pushing steps in CI workflow ([ce21611](https://github.com/inference-gateway/inference-gateway/commit/ce216118fd4ef1d76b6507cd3b07d02d0388b34c))
* Update release upload commands to remove redundant 'v' prefix for version ([255852e](https://github.com/inference-gateway/inference-gateway/commit/255852ea12c86bf6e82ef4d624218be159fd1871))

## [0.5.0-rc.3](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.2...v0.5.0-rc.3) (2025-04-10)

### ✨ Features

* Enhance release workflow with GitHub App token management and artifact uploads ([1dcb38d](https://github.com/inference-gateway/inference-gateway/commit/1dcb38d75859f24a54c113bf1bf9481d2de319eb))

### 🐛 Bug Fixes

* Include Chart.yaml in assets for semantic-release ([9a7a16a](https://github.com/inference-gateway/inference-gateway/commit/9a7a16a04ea82e4d28a2b19f771d7d095c064d41))

## [0.5.0-rc.2](https://github.com/inference-gateway/inference-gateway/compare/v0.5.0-rc.1...v0.5.0-rc.2) (2025-04-10)

### ✨ Features

* Add application version parameter to configuration ([c8d7057](https://github.com/inference-gateway/inference-gateway/commit/c8d7057b1e8883d19ddb312e23edc692818fb710))

### ♻️ Improvements

* Add String method to Config for improved string representation ([1d4dfda](https://github.com/inference-gateway/inference-gateway/commit/1d4dfdad0d360707522d25b6ecaeba6c38ee761a))
* Define application name and version constants; update String method to use constants ([60ffca5](https://github.com/inference-gateway/inference-gateway/commit/60ffca5b0399bfd306b257e54d8b689548b74a9c))
* Remove hardcoded application name from tests and update OpenTelemetry initialization to use constants ([0ade1c5](https://github.com/inference-gateway/inference-gateway/commit/0ade1c59fdcfc96309170513e9ee0f330aa78426))
* Remove VERSION constant and update String method to use APPLICATION_NAME constant ([3a225d4](https://github.com/inference-gateway/inference-gateway/commit/3a225d481f52dca961bbef9f57e183a377437d11))
* Run task generate - remove APPLICATION_NAME from configuration files and update related examples ([d061a84](https://github.com/inference-gateway/inference-gateway/commit/d061a8486d53496171afc567e63ff1acf06b32f5))

### 🐛 Bug Fixes

* Comment out temporarily Helm chart version update commands in release workflow ([79f99c0](https://github.com/inference-gateway/inference-gateway/commit/79f99c08f8e8f24a71526cf58993311a2a1e13f9))
* Correct indentation for prepareCmd in semantic-release configuration ([c2d4003](https://github.com/inference-gateway/inference-gateway/commit/c2d40036b16174be1cf08641d0953d14ca43f98f))
* Migrate the configuration of the linter to the latest ([1e838ed](https://github.com/inference-gateway/inference-gateway/commit/1e838ed3957be585df3a8b2aa05a011d59df3fd2))
* Update prepareCmd to reference meta.go instead of version.go in semantic-release configuration ([3dc5b7b](https://github.com/inference-gateway/inference-gateway/commit/3dc5b7bfd49f3d69343b029fc979ec99296ba51e))
* Update prepareCmd to set Helm chart version in Chart.yaml during release process ([82d222d](https://github.com/inference-gateway/inference-gateway/commit/82d222d157408760a449d875366a743ae40579a0))
* Update release assets to include version.go instead of config.go ([af509c6](https://github.com/inference-gateway/inference-gateway/commit/af509c66a1ecbec98e0afc82561715853396c218))

### 🔧 Miscellaneous

* Add debug logging for loaded configuration in debug and development environments ([36a22df](https://github.com/inference-gateway/inference-gateway/commit/36a22dfb328ccdd5df03c6a9be40c7293fd89339))
* update helm chart version to 0.5.0-rc.2 [skip ci] ([c34087b](https://github.com/inference-gateway/inference-gateway/commit/c34087b96e4cbbadcb7a9fd1a242ff1a46c14bed))
* Update Keycloak CA configmap creation to use server-side apply with dry-run so it could be executed and produce the same result in idempotent way ([9fa238e](https://github.com/inference-gateway/inference-gateway/commit/9fa238e918212f13d3c1c70978ba142978eed9a9))

### ✅ Miscellaneous

* Update image tag for inference-gateway deployment to 0.5.0-rc.1 ([0e7a88b](https://github.com/inference-gateway/inference-gateway/commit/0e7a88b52730244d7f23bee19cebbe762bec394d))

### 🔨 Miscellaneous

* Add semantic-release dry run to Taskfile for release process ([abf5ed8](https://github.com/inference-gateway/inference-gateway/commit/abf5ed8b7b040ddabec58cb27d3eafd371cb6569))

## [0.5.0-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.4.1...v0.5.0-rc.1) (2025-04-10)

### ✨ Features

* Add inference-gateway Helm chart with dependencies and monitoring support ([ebd9942](https://github.com/inference-gateway/inference-gateway/commit/ebd9942db53ea434067262ef6baf8a10acf9e23a))
* Add ingress-nginx dependency and update configurations in values.yaml, Chart.yaml, and Chart.lock default to false ([3a92e81](https://github.com/inference-gateway/inference-gateway/commit/3a92e81fe95607328a10dfc9929be24402355b0d))

### ♻️ Improvements

* Enable autoscaling for inference-gateway with updated max replicas ([69af532](https://github.com/inference-gateway/inference-gateway/commit/69af532f7d2c70d28e561b18b0699dc7e6ab63b9))
* Keep it simple - refactor code structure for improved readability and maintainability ([912361e](https://github.com/inference-gateway/inference-gateway/commit/912361e5fb8290c3825ae95c041da57ff6efb39a))

### 🐛 Bug Fixes

* Add missing installCRDs flag for cert-manager installation ([2211a0b](https://github.com/inference-gateway/inference-gateway/commit/2211a0b2ff07e6d27bd15fff0e78cb1ed561fdc5))
* Correct image tag formatting in deployment.yaml ([dd5b0b9](https://github.com/inference-gateway/inference-gateway/commit/dd5b0b9f059dac11eb9d6fa1041b4458b33013c5))
* Correct kube-prometheus-stack and grafana-operator configuration in values.yaml - this is how to update dependencies values ([0788cc2](https://github.com/inference-gateway/inference-gateway/commit/0788cc2c2abada7f00534a862117cd12224d320a))
* Remove default admin credentials for keycloakx installation ([b4fec38](https://github.com/inference-gateway/inference-gateway/commit/b4fec38662b6734de01e934ad6c27942b6c41251))
* Remove default fullnameOverride value in values.yaml ([11eec4f](https://github.com/inference-gateway/inference-gateway/commit/11eec4ff82b9a0b98d289b388c88b9303b5bb501))
* Remove fullnameOverride setting for inference-gateway installation ([318c89b](https://github.com/inference-gateway/inference-gateway/commit/318c89be51990f6d866e2acd7f773289e8440cdb))
* Update cert-manager installation flag from installCRDs to crds.enabled ([77d53fe](https://github.com/inference-gateway/inference-gateway/commit/77d53fe48f5175532e826f8313491f5ef5c04025))
* Update liveness and readiness probe paths to /health ([f063e3c](https://github.com/inference-gateway/inference-gateway/commit/f063e3c678af95b2c5e5844f981ff34ff2feaad6))
* Update liveness and readiness probe paths to /health and change ingress-nginx namespace to kube-system ([250153f](https://github.com/inference-gateway/inference-gateway/commit/250153fcc0ac2ef86d65e06e4116bbc9701b65c0))

### 🔧 Miscellaneous

* Add Bitnami PostgreSQL and update Keycloak configuration in Taskfile ([f39d232](https://github.com/inference-gateway/inference-gateway/commit/f39d23246c4d29aa9dc8269b94d870227ec0fbbd))
* Add import-realm task to Taskfile for Keycloak realm configuration ([f36fa18](https://github.com/inference-gateway/inference-gateway/commit/f36fa1846828e9a690205000bf00886a675d6faa))
* Add Keycloak resources and update ingress configuration in Taskfile ([abfc2e6](https://github.com/inference-gateway/inference-gateway/commit/abfc2e6054b78ff884dc1ec4c70252bf875a1737))
* Add more metadata to chart ([e838572](https://github.com/inference-gateway/inference-gateway/commit/e838572807eadc3475bfafc835a48f84eb7a24fd))
* Add new line at the end of the file ([c21fb92](https://github.com/inference-gateway/inference-gateway/commit/c21fb92073ca5f89118d1c14e5199f736638ac70))
* Add port configurations for loadbalancer in Cluster.yaml ([7845fae](https://github.com/inference-gateway/inference-gateway/commit/7845fae72a3e7db219d36680403e27fc4a4f7ea1))
* Add resource requests and limits for inference-gateway deployment ([fed2977](https://github.com/inference-gateway/inference-gateway/commit/fed2977975d0f2e668b561b6e3a8875b881565fd))
* Add self-signed certificate issuer and update ingress annotations for inference-gateway ([3bf0f8c](https://github.com/inference-gateway/inference-gateway/commit/3bf0f8c67161de14f8a9eeaf763846c7b7e85507))
* Hack refactor helm chart, fix the Configmap generation, add good defaults ([308bb0c](https://github.com/inference-gateway/inference-gateway/commit/308bb0c9894f9afd77e9fd69a14110722f52ff2c))
* Remove obsolete test.yaml file from hack directory ([4fdf981](https://github.com/inference-gateway/inference-gateway/commit/4fdf981e1a46e801a1cda42225faf877ccbd4a59))
* Remove persist-credentials option from GitHub checkout action ([45ad2f4](https://github.com/inference-gateway/inference-gateway/commit/45ad2f4738fd247cc7607c33c5ee7fd106ff460c))
* Update appPort and runArgs in devcontainer configuration ([c521601](https://github.com/inference-gateway/inference-gateway/commit/c521601e8511dcc88976a8bae48263e0ae139d75))
* Update grafana-operator version to v5.17.0 in Chart.yaml and Chart.lock ([29bcf76](https://github.com/inference-gateway/inference-gateway/commit/29bcf768732addc7e1ede610d98f29c5c1590a35))
* update helm chart version to 0.5.0-rc.1 [skip ci] ([0c834bf](https://github.com/inference-gateway/inference-gateway/commit/0c834bffe37c0e9b3ff1493c3a29d70e55cea26b))
* Update inference-gateway deployment with secrets and config map ([56ee561](https://github.com/inference-gateway/inference-gateway/commit/56ee561dd6ad9b71d81d4069c8b29336d99763ae))
* Update ingress host and TLS configuration for inference-gateway ([d6cfa25](https://github.com/inference-gateway/inference-gateway/commit/d6cfa250062f0ecf85c69b30a74db6e2f16e3ba8))
* Update Keycloak hostname and enhance helm deployment with CA trust configuration ([a7d4fa2](https://github.com/inference-gateway/inference-gateway/commit/a7d4fa2f779b75a2e5f29948ce8e28626b2340d1))
* Update Keycloak service references and remove obsolete port-forward task ([34758ee](https://github.com/inference-gateway/inference-gateway/commit/34758ee1948da1ab73cf76940f4c5f5945a04d64))
* Update resource names to use release-name-inference-gateway instead of ig ([ffc1b82](https://github.com/inference-gateway/inference-gateway/commit/ffc1b82942d05bf2ef9528f9aacaa599583db57b))
* Update resource requests and limits for inference-gateway ([35ebb55](https://github.com/inference-gateway/inference-gateway/commit/35ebb559b215314273fbe3b078045008d476faf2))

## [0.4.1](https://github.com/inference-gateway/inference-gateway/compare/v0.4.0...v0.4.1) (2025-04-06)

### 🐛 Bug Fixes

* Make the Inference-Gateway Clients Aware ([#58](https://github.com/inference-gateway/inference-gateway/issues/58)) ([4da9450](https://github.com/inference-gateway/inference-gateway/commit/4da94509e18ec823eb74a78f2d0b1e08088c6001))

## [0.4.0](https://github.com/inference-gateway/inference-gateway/compare/v0.3.1...v0.4.0) (2025-03-31)

### ✨ Features

* Add reasoning_content field to chunk message in OpenAPI specification ([#57](https://github.com/inference-gateway/inference-gateway/issues/57)) ([ff1c270](https://github.com/inference-gateway/inference-gateway/commit/ff1c270d3874a4419242283e83e407fd678182dd))

### 📚 Documentation

* Simplify Docker Compose UI example README and update setup instructions ([e871103](https://github.com/inference-gateway/inference-gateway/commit/e871103dd58aa9d9e48b987abfa1916ed4d2fe7b))
* Update API domain to api.inference-gateway.local and add Docker Compose UI example ([#55](https://github.com/inference-gateway/inference-gateway/issues/55)) ([7449a2d](https://github.com/inference-gateway/inference-gateway/commit/7449a2d8cc90ade416d8eb87b9d1a0d5f46c87c0))

## [0.3.1](https://github.com/inference-gateway/inference-gateway/compare/v0.3.0...v0.3.1) (2025-03-29)

### ♻️ Improvements

* Prefix model IDs with provider name for consistency across providers ([#56](https://github.com/inference-gateway/inference-gateway/issues/56)) ([5c2a752](https://github.com/inference-gateway/inference-gateway/commit/5c2a752ea920189de35110f45f37fcb373d95526))

## [0.3.0](https://github.com/inference-gateway/inference-gateway/compare/v0.2.22...v0.3.0) (2025-03-25)

### ✨ Features

* Add DeepSeek Provider ([#52](https://github.com/inference-gateway/inference-gateway/issues/52)) ([2dbdbeb](https://github.com/inference-gateway/inference-gateway/commit/2dbdbeb975a75ec888280ffc3968c14da1c39c9d))

### 📚 Documentation

* **openapi:** Update server definitions and paths for improved API structure ([#53](https://github.com/inference-gateway/inference-gateway/issues/53)) ([7816c02](https://github.com/inference-gateway/inference-gateway/commit/7816c0244552d89c08b62b338869f972f66209db))
* Update README to include H3 in provider class diagram ([082ede3](https://github.com/inference-gateway/inference-gateway/commit/082ede3a698bf7e7b410c3fc8136d534e7ed81ce))

## [0.2.22](https://github.com/inference-gateway/inference-gateway/compare/v0.2.21...v0.2.22) (2025-03-23)

### 📚 Documentation

* **openapi:** Improve schema ([#50](https://github.com/inference-gateway/inference-gateway/issues/50)) ([f1c6129](https://github.com/inference-gateway/inference-gateway/commit/f1c6129dec2de67765aeebffb459ba288d026a37))
* **openapi:** Remove nullable properties from schema definitions ([13530e2](https://github.com/inference-gateway/inference-gateway/commit/13530e2882c6f10f7987856994425e81b78f4de8))
* **openapi:** Remove required properties from CompletionUsage schema ([0f74205](https://github.com/inference-gateway/inference-gateway/commit/0f742057157a01a2d9066fb3d1d7329fe84a23d5))

### 🔧 Miscellaneous

* **docker-compose:** Update inference-gateway service to use pre-built image and remove commented-out configurations ([#49](https://github.com/inference-gateway/inference-gateway/issues/49)) ([e662799](https://github.com/inference-gateway/inference-gateway/commit/e662799e0cc635eb793dd54faf0f719b80011930))

## [0.2.21](https://github.com/inference-gateway/inference-gateway/compare/v0.2.20...v0.2.21) (2025-03-19)

### ⚠ BREAKING CHANGES

* **endpoints:** Those endpoints will no longer exists - transitioning to OpenAI compatible endpoints.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* docs(openapi): Resort the paths

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* docs: Update API endpoints in README for model retrieval and chat completions

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(routes): Remove ListAllModelsHandler and ListModelsHandler methods

These are deprecated in favor of a single handler v1/models

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor: Remove OpenAICompatible from the code names, keep it agnostic, just leave a docblock to inform it's compatible is enough

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* build: Update versions in Dockerfile and CI workflow for dependencies

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(routes): Enhance provider determination logic in ChatCompletionsHandler

Allow users to specify the prefix of the provider in the model name, simplifies configurations of Coding editors and extensions

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* docs(openapi): Add CreateCompletion endpoint and request/response schemas

Seems like the older version of this API, but some coding tools still using it, so adding the minimal of it for backward compatibility.

I assume it's like in Ollama API where you send tokens and you receive some tokens back - only the completion rather than the entire chat. Perhaps it's there for a reason because it's more efficient, less tokens needs to be passed over the connection wire. Curious to see how the IDE's is using it.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* chore(openapi): Correct path formatting for completions endpoint

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* docs(openai): Add the absolutely minimal endpoints and schemas needed

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* feat(api): Add CompletionsHandler endpoint for creating completions

TODO - I need to implement this so the IDE's like cursor and IDE's extensions like continue.dev will work.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* chore: Run task generate

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Remove unused CompletionsHandler and related OpenAPI definitions

This is a legacy endpoint and could be opt-out in different existing tools, so no need to implement it, probably for now.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* chore: Uncomment command to generate ProvidersCommonTypes in Taskfile

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* chore(codegen): Enhance code generation with new types and improved formatting logic

It works well, just need to make it work well with enums instead of hard-coding them, but for now will just hardcode them.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update base URLs for providers to include versioning

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Simplify descriptions in OpenAPI definitions for clarity

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Mark ResponseTokens and GenerateResponse as deprecated

Will remove them at the end on the cleanup stage, still need to refactor GenerateResponseTokens function.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update endpoint structure to use 'Models' and 'Chat' instead of 'List' and 'Generate'

Also remove dead code, functions for generating providers that I wanted to use but I never completed, I'll probably get back to it once there is a clear process.
Many providers now don't even need specific mutations since they became compatible with OpenAI.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Mark GenerateRequest as deprecated in OpenAPI definition

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update provider API URLs to include versioning and enhance endpoint structure

Re-implement chat completion endpoint for all providers, it's now fully OpenAI compatible.

Some providers like cohere and cloudflare didn't make the /v1/models end point compatible, only the /v1/chat/completions endpoint, which is kind of weird, so I had to create an extra compatibility also for the list endpoint.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* chore(api): Clean up too many comments, leave only the essentials, the code is self explanatory

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Simplify streaming API

Removed some unnecessary logic, since most providers have now OpenAI compatibility API that means no extra logic needs to be implemented, sending the response back to the client as-is, which results in better performance.

Still need to test Ollama, when the server is timing out the streaming is being closed unexpectedly, I didn't figure out how to increase the timeout on Ollama server, since it's running locally it takes longer to generate the tokens. I did find out in their documentation how to enable Debugging messages, and it's a bit clearer that my request was failing due to timeout on Ollama server. It's nowhere documented so I'll have to figure out.

Other providers works almost seamlessly. Need to write new tests with this OpenAI compatible API.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* chore(wip): Need to test the monitoring

Will probably continue tomorrow, seems like most providers provide token counts but not the total time etc, it's not in OpenAI specifications - only Groq provide it, I'll comment it out for now.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Enhance JSON tags and default values in API models

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update telemetry middleware and Kubernetes configurations for improved provider detection and Ollama service port

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update JSON tags to use default values properly

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Add debug logging for unknown provider in telemetry middleware

Avoid spamming the dashboard, with metrics of unknown provider, just throw a Debug message in case this happens.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Sort the commented out latency checks, will deal with this later

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update token usage handling in telemetry middleware and adjust types in OpenAPI and common types

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(docs): Simplify usage section in REST endpoints README by removing unused fields

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Enhance telemetry middleware to handle streaming responses and improve token extraction logic

I had to parse the stream in order to get the usage in the final message properly.

TODO - I need to figure out why Ollama OpenAI compatible API is not sending the usage on the final message.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Remove commented-out latency checks in telemetry middleware

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update example response content in ChatCompletionsHandler for clarity and conciseness

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Update README examples and OpenAPI schema to reflect changes in chat completions and tool call structure

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Remove default tags from go structs and set the values explicitly

Also add stream_options, so we can set ollama default token usage to true, it's set to false by default and not shown on the final SSE message - by setting it explicitly to true we get token usages from all providers.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Change model slices to use pointers for improved memory efficiency and compatibility

Cohere apparently haven't implemented stream_options so when we send the request with stream_options attributes in the payload we get 422 Unprocessable entity status. Kind of weird that they call it "Compatible" API because it's clearly not. Anyways that would be a quick patch for this special case.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Cloudflare like Cohere did a partial job in implementing compatibility for OpenAI - fixing it

Also checking for nil Usage since it's a pointer now and will be omitted on some cases.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Improve telemetry middleware logging and enforce usage tracking for streaming completions

For Usage metrics I just need the last 2 messages, this improve the speed of parsing the usage.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(docs): Update REST API examples to reflect changes in response structure and remove streaming option

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* refactor(api): Increase limit to 1000 of models in cloudflare listing of public finetuned LLMs

Strange I remember they had more LLMs but now they only serve 3

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* test(api): Add unit tests for provider registry and chat completions functionality

Signed-off-by: Eden Reich <eden.reich@gmail.com>

* chore(release): 🔖 0.3.0-rc.1 [skip ci]

## [0.3.0-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.2.20...v0.3.0-rc.1) (2025-03-19)

### ⚠ BREAKING CHANGES

* **docs:** Those endpoints will no longer exists - transitioning to OpenAI compatible endpoints.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

### ✨ Features

* **api:** Add CompletionsHandler endpoint for creating completions ([a950072](https://github.com/inference-gateway/inference-gateway/commit/a950072c7ae2d4ddcdbf765439b4040df14393be))

### ♻️ Improvements

* **api:** Add debug logging for unknown provider in telemetry middleware ([e71a596](https://github.com/inference-gateway/inference-gateway/commit/e71a596d70719778152df6180ddf13d4dc427461))
* **api:** Change model slices to use pointers for improved memory efficiency and compatibility ([8e36717](https://github.com/inference-gateway/inference-gateway/commit/8e36717a5bd0f273ece41dcd2c8181296a0f9e59))
* **api:** Cloudflare like Cohere did a partial job in implementing compatibility for OpenAI - fixing it ([971f12a](https://github.com/inference-gateway/inference-gateway/commit/971f12a171baee76513159277d333bcc9adcad3b))
* **api:** Enhance JSON tags and default values in API models ([2b3653e](https://github.com/inference-gateway/inference-gateway/commit/2b3653ea1cb57537a3953ef25cb340a3f5a9ce6b))
* **api:** Enhance telemetry middleware to handle streaming responses and improve token extraction logic ([1cd6500](https://github.com/inference-gateway/inference-gateway/commit/1cd650023d82123577d7a7a929ea7c45b2c69257))
* **api:** Improve telemetry middleware logging and enforce usage tracking for streaming completions ([d337dc6](https://github.com/inference-gateway/inference-gateway/commit/d337dc635caf9834274ff7528adedec244f4bfee))
* **api:** Increase limit to 1000 of models in cloudflare listing of public finetuned LLMs ([3279d1c](https://github.com/inference-gateway/inference-gateway/commit/3279d1cc11811ab7ae48e9dbf868b5b5ce9478bb))
* **api:** Mark GenerateRequest as deprecated in OpenAPI definition ([e7792ce](https://github.com/inference-gateway/inference-gateway/commit/e7792ce2233f6fe11fdb84eecc3d0805908a7ed3))
* **api:** Mark ResponseTokens and GenerateResponse as deprecated ([2b7fab0](https://github.com/inference-gateway/inference-gateway/commit/2b7fab04e6cd6a70d857a0c1f446d5b01bd0b227))
* **api:** Remove commented-out latency checks in telemetry middleware ([db1caec](https://github.com/inference-gateway/inference-gateway/commit/db1caece232ff708eaadfd5481aacae1fbee42d5))
* **api:** Remove default tags from go structs and set the values explicitly ([de9a778](https://github.com/inference-gateway/inference-gateway/commit/de9a778d47b3bd7c724fdf87d492e481c2eea9d7))
* **api:** Remove unused CompletionsHandler and related OpenAPI definitions ([0de13e2](https://github.com/inference-gateway/inference-gateway/commit/0de13e2c7550969b5a14d0992e12f58d1f22eaac))
* **api:** Simplify descriptions in OpenAPI definitions for clarity ([78e0d56](https://github.com/inference-gateway/inference-gateway/commit/78e0d56d620392fdf2f29485d73fec491e98a284))
* **api:** Simplify streaming API ([a736906](https://github.com/inference-gateway/inference-gateway/commit/a73690657fdd10151fbdb4a33ec47e854f139399))
* **api:** Sort the commented out latency checks, will deal with this later ([e8539fd](https://github.com/inference-gateway/inference-gateway/commit/e8539fdc0a41519ddf91b16059c0adaf26dbffce))
* **api:** Update base URLs for providers to include versioning ([abf4ba4](https://github.com/inference-gateway/inference-gateway/commit/abf4ba40698ee97ff257dcb9fa0672023f66d2c1))
* **api:** Update endpoint structure to use 'Models' and 'Chat' instead of 'List' and 'Generate' ([6c6927c](https://github.com/inference-gateway/inference-gateway/commit/6c6927c7f348016a0d065da476b64035e65dff87))
* **api:** Update example response content in ChatCompletionsHandler for clarity and conciseness ([70aa99b](https://github.com/inference-gateway/inference-gateway/commit/70aa99ba5c5e91a29186cec8a98afb04914f8c84))
* **api:** Update JSON tags to use default values properly ([f2a31b9](https://github.com/inference-gateway/inference-gateway/commit/f2a31b94fd7e8aea335bb152cc6f202902386ddc))
* **api:** Update provider API URLs to include versioning and enhance endpoint structure ([9c2bac1](https://github.com/inference-gateway/inference-gateway/commit/9c2bac1a9441d1af2f7ab19c106197f7df46d111))
* **api:** Update README examples and OpenAPI schema to reflect changes in chat completions and tool call structure ([3701968](https://github.com/inference-gateway/inference-gateway/commit/37019683518f602ede5463844954825d9b47af2b))
* **api:** Update telemetry middleware and Kubernetes configurations for improved provider detection and Ollama service port ([a111768](https://github.com/inference-gateway/inference-gateway/commit/a11176839c4bb19fab7379956225ef11a96d0323))
* **api:** Update token usage handling in telemetry middleware and adjust types in OpenAPI and common types ([550c61c](https://github.com/inference-gateway/inference-gateway/commit/550c61c8e7aa42732641e9610d5a7e38ec2d16ac))
* **docs:** Remove deprecated LLM endpoints from OpenAPI specification ([46eda1a](https://github.com/inference-gateway/inference-gateway/commit/46eda1a5f1853ef613e521de4ff830f3b974a469))
* **docs:** Simplify usage section in REST endpoints README by removing unused fields ([a47587b](https://github.com/inference-gateway/inference-gateway/commit/a47587bd4a0023efa9e27310f467c8259ec195be))
* **docs:** Update REST API examples to reflect changes in response structure and remove streaming option ([c287652](https://github.com/inference-gateway/inference-gateway/commit/c287652dce04c85c7b8f1d364ff31ae6968c7240))
* Enhance ListModelsOpenAICompatibleHandler to support multiple providers and improve error handling ([76eb371](https://github.com/inference-gateway/inference-gateway/commit/76eb3716b5193711724f182dee869c42652614e4))
* Remove OpenAICompatible from the code names, keep it agnostic, just leave a docblock to inform it's compatible is enough ([c09ebab](https://github.com/inference-gateway/inference-gateway/commit/c09ebab3102d8dc44547c3dbb77508474202a98a))
* Rename GenerateRequest to ChatCompletionsRequest and update related transformations across providers ([f87f77b](https://github.com/inference-gateway/inference-gateway/commit/f87f77b90e157ccf760c3fb86b0265fd514905e0))
* **routes:** Enhance provider determination logic in ChatCompletionsHandler ([86fcc37](https://github.com/inference-gateway/inference-gateway/commit/86fcc373288a45cf0625433e0bc4d450127ba104))
* **routes:** Remove ListAllModelsHandler and ListModelsHandler methods ([d46766a](https://github.com/inference-gateway/inference-gateway/commit/d46766a092e2c6f1e91f2f092691b8f056e7ee83))
* Run task generate ([7337868](https://github.com/inference-gateway/inference-gateway/commit/73378687a66679668117608d41efd03964a446fc))
* Update model response structure to use 'Data' and 'Object' fields same as in OpenAI ([5a4fdb7](https://github.com/inference-gateway/inference-gateway/commit/5a4fdb78599ec4c56aa33cc0a011228e5c32c1bc))

### 🐛 Bug Fixes

* **tests:** Update ListModels response structure to include 'Object' and 'OwnedBy' fields ([38004af](https://github.com/inference-gateway/inference-gateway/commit/38004afbb4eb3a76af41ed5ce9dc0111410c3757))

### 📚 Documentation

* **api:** Add ChatCompletionsOpenAICompatibleHandler for OpenAI-compatible text completions ([e401164](https://github.com/inference-gateway/inference-gateway/commit/e40116496762b52f730e2757a8b3b776b6457313))
* **openai:** Add the absolutely minimal endpoints and schemas needed ([2ace1d6](https://github.com/inference-gateway/inference-gateway/commit/2ace1d61a37caa092f31a2b1687f5995c477c953))
* **openapi:** Add CreateCompletion endpoint and request/response schemas ([6871bf3](https://github.com/inference-gateway/inference-gateway/commit/6871bf3e26adde400eceef014d7327649e2e85ce))
* **openapi:** Resort the paths ([f06009d](https://github.com/inference-gateway/inference-gateway/commit/f06009d7b651baf10e005e621686c509c33d1050))
* Update API endpoints in README for model retrieval and chat completions ([40157f9](https://github.com/inference-gateway/inference-gateway/commit/40157f9a663e21e13a2ba0a3e818a321172fda8d))
* Update example API request in README for chat completions ([9df10f9](https://github.com/inference-gateway/inference-gateway/commit/9df10f9320d3d5f3c562ce21e83a715d0fa46136))
* Update ListModelsOpenAICompatibleHandler documentation to clarify endpoint usage and response format ([b8fa8eb](https://github.com/inference-gateway/inference-gateway/commit/b8fa8eb21a34c512fd9a21442dad78b47697ee41))
* Update README with new endpoint URLs for model listing ([a3798a4](https://github.com/inference-gateway/inference-gateway/commit/a3798a431cecaa88214447d6804439ddb2cc1853))

### 🔧 Miscellaneous

* **api:** Clean up too many comments, leave only the essentials, the code is self explanatory ([fc6dafe](https://github.com/inference-gateway/inference-gateway/commit/fc6dafece932076e2230d17b3c86e6a9bc306919))
* **codegen:** Enhance code generation with new types and improved formatting logic ([10dc8f4](https://github.com/inference-gateway/inference-gateway/commit/10dc8f4c4940a70acc0b9aff56eeb78fb97a55f8))
* **openapi:** Correct path formatting for completions endpoint ([c15d27a](https://github.com/inference-gateway/inference-gateway/commit/c15d27a6f7c705f605e198077a8b8791953c77a8))
* Run task generate ([50b0bf6](https://github.com/inference-gateway/inference-gateway/commit/50b0bf6131759a1a967c945a35c7084b36e63cab))
* Run task generate ([37bdd36](https://github.com/inference-gateway/inference-gateway/commit/37bdd36e17695acb53508a52aed7ae2878f0f32b))
* Uncomment command to generate ProvidersCommonTypes in Taskfile ([76f608e](https://github.com/inference-gateway/inference-gateway/commit/76f608e219e2b098fc110431b2b9e7f029a67ed1))
* **wip:** Need to test the monitoring ([3b5de6d](https://github.com/inference-gateway/inference-gateway/commit/3b5de6d2eaff97efa07ba5063b9e5cfdca7f69ca))

### ✅ Miscellaneous

* Add additional tests to routes and break down tests by route ([d6504ce](https://github.com/inference-gateway/inference-gateway/commit/d6504ce292c7b6d06ba44556a38751467ac5cf2f))
* **api:** Add unit tests for provider registry and chat completions functionality ([f6764c5](https://github.com/inference-gateway/inference-gateway/commit/f6764c5fa7de874a5005e4f4a0a31075a5a58441))

### 🔨 Miscellaneous

* Update versions in Dockerfile and CI workflow for dependencies ([08269a2](https://github.com/inference-gateway/inference-gateway/commit/08269a28cbeaffd5552933c2be1f3d7336b25fa4))

### 🐛 Bug Fixes

* **endpoints:** Make the Inference Gateway more compatible with existing software that supports OpenAI  ([#45](https://github.com/inference-gateway/inference-gateway/issues/45)) ([2c0b13c](https://github.com/inference-gateway/inference-gateway/commit/2c0b13c0d14107fea5091a6a1d20a8533cda1911))

## [0.3.0-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.2.20...v0.3.0-rc.1) (2025-03-19)

### ⚠ BREAKING CHANGES

* **docs:** Those endpoints will no longer exists - transitioning to OpenAI compatible endpoints.

Signed-off-by: Eden Reich <eden.reich@gmail.com>

### ✨ Features

* **api:** Add CompletionsHandler endpoint for creating completions ([a950072](https://github.com/inference-gateway/inference-gateway/commit/a950072c7ae2d4ddcdbf765439b4040df14393be))

### ♻️ Improvements

* **api:** Add debug logging for unknown provider in telemetry middleware ([e71a596](https://github.com/inference-gateway/inference-gateway/commit/e71a596d70719778152df6180ddf13d4dc427461))
* **api:** Change model slices to use pointers for improved memory efficiency and compatibility ([8e36717](https://github.com/inference-gateway/inference-gateway/commit/8e36717a5bd0f273ece41dcd2c8181296a0f9e59))
* **api:** Cloudflare like Cohere did a partial job in implementing compatibility for OpenAI - fixing it ([971f12a](https://github.com/inference-gateway/inference-gateway/commit/971f12a171baee76513159277d333bcc9adcad3b))
* **api:** Enhance JSON tags and default values in API models ([2b3653e](https://github.com/inference-gateway/inference-gateway/commit/2b3653ea1cb57537a3953ef25cb340a3f5a9ce6b))
* **api:** Enhance telemetry middleware to handle streaming responses and improve token extraction logic ([1cd6500](https://github.com/inference-gateway/inference-gateway/commit/1cd650023d82123577d7a7a929ea7c45b2c69257))
* **api:** Improve telemetry middleware logging and enforce usage tracking for streaming completions ([d337dc6](https://github.com/inference-gateway/inference-gateway/commit/d337dc635caf9834274ff7528adedec244f4bfee))
* **api:** Increase limit to 1000 of models in cloudflare listing of public finetuned LLMs ([3279d1c](https://github.com/inference-gateway/inference-gateway/commit/3279d1cc11811ab7ae48e9dbf868b5b5ce9478bb))
* **api:** Mark GenerateRequest as deprecated in OpenAPI definition ([e7792ce](https://github.com/inference-gateway/inference-gateway/commit/e7792ce2233f6fe11fdb84eecc3d0805908a7ed3))
* **api:** Mark ResponseTokens and GenerateResponse as deprecated ([2b7fab0](https://github.com/inference-gateway/inference-gateway/commit/2b7fab04e6cd6a70d857a0c1f446d5b01bd0b227))
* **api:** Remove commented-out latency checks in telemetry middleware ([db1caec](https://github.com/inference-gateway/inference-gateway/commit/db1caece232ff708eaadfd5481aacae1fbee42d5))
* **api:** Remove default tags from go structs and set the values explicitly ([de9a778](https://github.com/inference-gateway/inference-gateway/commit/de9a778d47b3bd7c724fdf87d492e481c2eea9d7))
* **api:** Remove unused CompletionsHandler and related OpenAPI definitions ([0de13e2](https://github.com/inference-gateway/inference-gateway/commit/0de13e2c7550969b5a14d0992e12f58d1f22eaac))
* **api:** Simplify descriptions in OpenAPI definitions for clarity ([78e0d56](https://github.com/inference-gateway/inference-gateway/commit/78e0d56d620392fdf2f29485d73fec491e98a284))
* **api:** Simplify streaming API ([a736906](https://github.com/inference-gateway/inference-gateway/commit/a73690657fdd10151fbdb4a33ec47e854f139399))
* **api:** Sort the commented out latency checks, will deal with this later ([e8539fd](https://github.com/inference-gateway/inference-gateway/commit/e8539fdc0a41519ddf91b16059c0adaf26dbffce))
* **api:** Update base URLs for providers to include versioning ([abf4ba4](https://github.com/inference-gateway/inference-gateway/commit/abf4ba40698ee97ff257dcb9fa0672023f66d2c1))
* **api:** Update endpoint structure to use 'Models' and 'Chat' instead of 'List' and 'Generate' ([6c6927c](https://github.com/inference-gateway/inference-gateway/commit/6c6927c7f348016a0d065da476b64035e65dff87))
* **api:** Update example response content in ChatCompletionsHandler for clarity and conciseness ([70aa99b](https://github.com/inference-gateway/inference-gateway/commit/70aa99ba5c5e91a29186cec8a98afb04914f8c84))
* **api:** Update JSON tags to use default values properly ([f2a31b9](https://github.com/inference-gateway/inference-gateway/commit/f2a31b94fd7e8aea335bb152cc6f202902386ddc))
* **api:** Update provider API URLs to include versioning and enhance endpoint structure ([9c2bac1](https://github.com/inference-gateway/inference-gateway/commit/9c2bac1a9441d1af2f7ab19c106197f7df46d111))
* **api:** Update README examples and OpenAPI schema to reflect changes in chat completions and tool call structure ([3701968](https://github.com/inference-gateway/inference-gateway/commit/37019683518f602ede5463844954825d9b47af2b))
* **api:** Update telemetry middleware and Kubernetes configurations for improved provider detection and Ollama service port ([a111768](https://github.com/inference-gateway/inference-gateway/commit/a11176839c4bb19fab7379956225ef11a96d0323))
* **api:** Update token usage handling in telemetry middleware and adjust types in OpenAPI and common types ([550c61c](https://github.com/inference-gateway/inference-gateway/commit/550c61c8e7aa42732641e9610d5a7e38ec2d16ac))
* **docs:** Remove deprecated LLM endpoints from OpenAPI specification ([46eda1a](https://github.com/inference-gateway/inference-gateway/commit/46eda1a5f1853ef613e521de4ff830f3b974a469))
* **docs:** Simplify usage section in REST endpoints README by removing unused fields ([a47587b](https://github.com/inference-gateway/inference-gateway/commit/a47587bd4a0023efa9e27310f467c8259ec195be))
* **docs:** Update REST API examples to reflect changes in response structure and remove streaming option ([c287652](https://github.com/inference-gateway/inference-gateway/commit/c287652dce04c85c7b8f1d364ff31ae6968c7240))
* Enhance ListModelsOpenAICompatibleHandler to support multiple providers and improve error handling ([76eb371](https://github.com/inference-gateway/inference-gateway/commit/76eb3716b5193711724f182dee869c42652614e4))
* Remove OpenAICompatible from the code names, keep it agnostic, just leave a docblock to inform it's compatible is enough ([c09ebab](https://github.com/inference-gateway/inference-gateway/commit/c09ebab3102d8dc44547c3dbb77508474202a98a))
* Rename GenerateRequest to ChatCompletionsRequest and update related transformations across providers ([f87f77b](https://github.com/inference-gateway/inference-gateway/commit/f87f77b90e157ccf760c3fb86b0265fd514905e0))
* **routes:** Enhance provider determination logic in ChatCompletionsHandler ([86fcc37](https://github.com/inference-gateway/inference-gateway/commit/86fcc373288a45cf0625433e0bc4d450127ba104))
* **routes:** Remove ListAllModelsHandler and ListModelsHandler methods ([d46766a](https://github.com/inference-gateway/inference-gateway/commit/d46766a092e2c6f1e91f2f092691b8f056e7ee83))
* Run task generate ([7337868](https://github.com/inference-gateway/inference-gateway/commit/73378687a66679668117608d41efd03964a446fc))
* Update model response structure to use 'Data' and 'Object' fields same as in OpenAI ([5a4fdb7](https://github.com/inference-gateway/inference-gateway/commit/5a4fdb78599ec4c56aa33cc0a011228e5c32c1bc))

### 🐛 Bug Fixes

* **tests:** Update ListModels response structure to include 'Object' and 'OwnedBy' fields ([38004af](https://github.com/inference-gateway/inference-gateway/commit/38004afbb4eb3a76af41ed5ce9dc0111410c3757))

### 📚 Documentation

* **api:** Add ChatCompletionsOpenAICompatibleHandler for OpenAI-compatible text completions ([e401164](https://github.com/inference-gateway/inference-gateway/commit/e40116496762b52f730e2757a8b3b776b6457313))
* **openai:** Add the absolutely minimal endpoints and schemas needed ([2ace1d6](https://github.com/inference-gateway/inference-gateway/commit/2ace1d61a37caa092f31a2b1687f5995c477c953))
* **openapi:** Add CreateCompletion endpoint and request/response schemas ([6871bf3](https://github.com/inference-gateway/inference-gateway/commit/6871bf3e26adde400eceef014d7327649e2e85ce))
* **openapi:** Resort the paths ([f06009d](https://github.com/inference-gateway/inference-gateway/commit/f06009d7b651baf10e005e621686c509c33d1050))
* Update API endpoints in README for model retrieval and chat completions ([40157f9](https://github.com/inference-gateway/inference-gateway/commit/40157f9a663e21e13a2ba0a3e818a321172fda8d))
* Update example API request in README for chat completions ([9df10f9](https://github.com/inference-gateway/inference-gateway/commit/9df10f9320d3d5f3c562ce21e83a715d0fa46136))
* Update ListModelsOpenAICompatibleHandler documentation to clarify endpoint usage and response format ([b8fa8eb](https://github.com/inference-gateway/inference-gateway/commit/b8fa8eb21a34c512fd9a21442dad78b47697ee41))
* Update README with new endpoint URLs for model listing ([a3798a4](https://github.com/inference-gateway/inference-gateway/commit/a3798a431cecaa88214447d6804439ddb2cc1853))

### 🔧 Miscellaneous

* **api:** Clean up too many comments, leave only the essentials, the code is self explanatory ([fc6dafe](https://github.com/inference-gateway/inference-gateway/commit/fc6dafece932076e2230d17b3c86e6a9bc306919))
* **codegen:** Enhance code generation with new types and improved formatting logic ([10dc8f4](https://github.com/inference-gateway/inference-gateway/commit/10dc8f4c4940a70acc0b9aff56eeb78fb97a55f8))
* **openapi:** Correct path formatting for completions endpoint ([c15d27a](https://github.com/inference-gateway/inference-gateway/commit/c15d27a6f7c705f605e198077a8b8791953c77a8))
* Run task generate ([50b0bf6](https://github.com/inference-gateway/inference-gateway/commit/50b0bf6131759a1a967c945a35c7084b36e63cab))
* Run task generate ([37bdd36](https://github.com/inference-gateway/inference-gateway/commit/37bdd36e17695acb53508a52aed7ae2878f0f32b))
* Uncomment command to generate ProvidersCommonTypes in Taskfile ([76f608e](https://github.com/inference-gateway/inference-gateway/commit/76f608e219e2b098fc110431b2b9e7f029a67ed1))
* **wip:** Need to test the monitoring ([3b5de6d](https://github.com/inference-gateway/inference-gateway/commit/3b5de6d2eaff97efa07ba5063b9e5cfdca7f69ca))

### ✅ Miscellaneous

* Add additional tests to routes and break down tests by route ([d6504ce](https://github.com/inference-gateway/inference-gateway/commit/d6504ce292c7b6d06ba44556a38751467ac5cf2f))
* **api:** Add unit tests for provider registry and chat completions functionality ([f6764c5](https://github.com/inference-gateway/inference-gateway/commit/f6764c5fa7de874a5005e4f4a0a31075a5a58441))

### 🔨 Miscellaneous

* Update versions in Dockerfile and CI workflow for dependencies ([08269a2](https://github.com/inference-gateway/inference-gateway/commit/08269a28cbeaffd5552933c2be1f3d7336b25fa4))

## [0.2.20](https://github.com/inference-gateway/inference-gateway/compare/v0.2.19...v0.2.20) (2025-03-10)

### 👷 CI

* **cleanup:** Remove redundant step in workflow ([#43](https://github.com/inference-gateway/inference-gateway/issues/43)) ([95c083e](https://github.com/inference-gateway/inference-gateway/commit/95c083e3eb4f559edfaff7dce7fb3f6046e62d71))

## [0.2.20-rc.3](https://github.com/inference-gateway/inference-gateway/compare/v0.2.20-rc.2...v0.2.20-rc.3) (2025-03-10)

### 👷 CI

* **release:** update semantic-release to version 24.2.3 ([d22d27d](https://github.com/inference-gateway/inference-gateway/commit/d22d27dfa88cbc790247a661fd67fe446ec04207))

## [0.2.20-rc.2](https://github.com/inference-gateway/inference-gateway/compare/v0.2.20-rc.1...v0.2.20-rc.2) (2025-03-10)

### 👷 CI

* **release:** Remove git user configuration step from release workflow ([5f032e7](https://github.com/inference-gateway/inference-gateway/commit/5f032e703f9fd4b028956bb3f8f2058c37859b13))

## [0.2.20-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.2.19...v0.2.20-rc.1) (2025-03-10)

### 👷 CI

* **release:** Add output logging for version determination error ([125c5df](https://github.com/inference-gateway/inference-gateway/commit/125c5df680f2b733e32257f22579c4cc775ae07b))
* **release:** Enable caching for Go setup in release workflow ([f99aa3b](https://github.com/inference-gateway/inference-gateway/commit/f99aa3bd0fadfe1967c3b168ab5908655710911e))
* **release:** Remove git user configuration step from workflow ([1d70b32](https://github.com/inference-gateway/inference-gateway/commit/1d70b3264980ab328f4c37f18be80393ef4d31f6))
* **release:** Revert, check whether this was breaking ([479f678](https://github.com/inference-gateway/inference-gateway/commit/479f6782e55c017c5ea4b11fe1b348ee75d77c0e))
* **release:** Use variable for bot email in release workflow ([b9ac6af](https://github.com/inference-gateway/inference-gateway/commit/b9ac6afef86a8f408e20196cf7a95e65d2855c56))

## [0.2.19](https://github.com/inference-gateway/inference-gateway/compare/v0.2.18...v0.2.19) (2025-03-10)

### 👷 CI

* **release:** Add container image scanning and signing steps to release workflow ([#41](https://github.com/inference-gateway/inference-gateway/issues/41)) ([a87895a](https://github.com/inference-gateway/inference-gateway/commit/a87895acaf29fc74c2e81859741e9ce99855f9c1))

### 🔧 Miscellaneous

* **todo:** Add step to sign container images in release workflow ([1df3cc0](https://github.com/inference-gateway/inference-gateway/commit/1df3cc0e6f138c042e69cf85199dba9deb24e83a))

## [0.2.19-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.2.18...v0.2.19-rc.1) (2025-03-10)

### 👷 CI

* **release:** Add container image scanning and signing steps to release workflow ([46071f1](https://github.com/inference-gateway/inference-gateway/commit/46071f1d7ac30f9c7b292cc2bc7e50dbed20852c))

### 🔧 Miscellaneous

* **todo:** Add step to sign container images in release workflow ([1df3cc0](https://github.com/inference-gateway/inference-gateway/commit/1df3cc0e6f138c042e69cf85199dba9deb24e83a))

## [0.2.18](https://github.com/inference-gateway/inference-gateway/compare/v0.2.17...v0.2.18) (2025-03-10)

### 🐛 Bug Fixes

* **release:** Correct version extraction regex to include 'v' prefix ([79a910f](https://github.com/inference-gateway/inference-gateway/commit/79a910f54120bd3b7f2d73756d55f8d12b19dcf6))
* **release:** Update version extraction to remove 'v' prefix and adjust image push command ([3e99f6f](https://github.com/inference-gateway/inference-gateway/commit/3e99f6f634d45618adffae2caa8ccb8be044a888))

## [0.2.17](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16...v0.2.17) (2025-03-10)

### 👷 CI

* Add step to push container images in release workflow ([e72c480](https://github.com/inference-gateway/inference-gateway/commit/e72c480e59531931aec286d0f74e8fe8f6e84b3e))

## [0.2.16](https://github.com/inference-gateway/inference-gateway/compare/v0.2.15...v0.2.16) (2025-03-10)

### 👷 CI

* Change release trigger to manual workflow dispatch ([a93ff17](https://github.com/inference-gateway/inference-gateway/commit/a93ff172f8d34b398ba55da3c127b68339a83a3b))
* Improve release workflow with semantic release auto versioning and changelog generation ([#40](https://github.com/inference-gateway/inference-gateway/issues/40)) ([cd7045c](https://github.com/inference-gateway/inference-gateway/commit/cd7045cd5c7c15990be8ff173f497653730d44ec))

### 📚 Documentation

* Add tool-use support and streaming responses to README ([af990a3](https://github.com/inference-gateway/inference-gateway/commit/af990a371142b726142beb06a08a11552a7abc58))
* Enhance diagram in README ([f98c3ff](https://github.com/inference-gateway/inference-gateway/commit/f98c3ff37b4084daa734c4b812598df60654afb8))
* **examples:** Add TLS configuration setup example for Inference Gateway ([#39](https://github.com/inference-gateway/inference-gateway/issues/39)) ([844df89](https://github.com/inference-gateway/inference-gateway/commit/844df89a5e335419e3d62e1d9240016e36c485d8))
* **fix:** Table was broken, fixed it ([a059a78](https://github.com/inference-gateway/inference-gateway/commit/a059a780f18c06eb2d1c2967e7c6d34fbe8921fd))
* Update OpenTelemetry description for clarity, keep it short and concise ([bd51700](https://github.com/inference-gateway/inference-gateway/commit/bd5170064d20869191c8e35aba5c4f4123ab1994))

## [0.2.16-rc.13](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.12...v0.2.16-rc.13) (2025-03-10)

### 🔨 Miscellaneous

* Add completions for GoReleaser and GitHub CLI in Zsh configuration ([ae70f5b](https://github.com/inference-gateway/inference-gateway/commit/ae70f5b364a38dfd80eab7f916b02d8752824709))
* Update archive formats in GoReleaser configuration ([8021a3b](https://github.com/inference-gateway/inference-gateway/commit/8021a3bd2b6f879644619616a58e8c643f109293))

## [0.2.16-rc.12](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.11...v0.2.16-rc.12) (2025-03-10)

### 👷 CI

* Update Docker login method for GitHub Container Registry ([4fc2ddf](https://github.com/inference-gateway/inference-gateway/commit/4fc2ddfd8805d5435b800d1d1f91f9ef0fd7c0d2))
* Update GoReleaser version to v2.7.0 in Dockerfile and release workflow ([7e2ab47](https://github.com/inference-gateway/inference-gateway/commit/7e2ab47cfe9155cc5ba70644b06f426cf7207c59))

## [0.2.16-rc.11](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.10...v0.2.16-rc.11) (2025-03-10)

### 👷 CI

* Remove fetching of latest tags from release workflow ([88a231b](https://github.com/inference-gateway/inference-gateway/commit/88a231b5778e7ef8b97de07b48b0575cfdeb9b1d))

## [0.2.16-rc.10](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.9...v0.2.16-rc.10) (2025-03-10)

### 👷 CI

* Move all permissions to the top and configure gpg key for verified commits by bot ([f1731d8](https://github.com/inference-gateway/inference-gateway/commit/f1731d81cbd0bcaf4db60c175d2e4da25154048c))

## [0.2.16-rc.9](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.8...v0.2.16-rc.9) (2025-03-10)

### 👷 CI

* Enhance release workflow to skip directories during upload and conditionally upload checksums ([be141fa](https://github.com/inference-gateway/inference-gateway/commit/be141fa5b4e5c9810368ec37f953d845fdf0050e))

## [0.2.16-rc.8](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.7...v0.2.16-rc.8) (2025-03-10)

### 👷 CI

* Add permissions for package management in release workflow ([398df4d](https://github.com/inference-gateway/inference-gateway/commit/398df4dca292f3fc35b81581f50d7a16d91d62fd))
* Update release workflow to skip announce and publish, and upload artifacts ([2722bdb](https://github.com/inference-gateway/inference-gateway/commit/2722bdbd5566792252a5dc9de4f06870ec3392fa))

### 🔧 Miscellaneous

* **goreleaser:** Update release mode to keep existing release created by semantic-release ([5424528](https://github.com/inference-gateway/inference-gateway/commit/5424528c2d50d1881a0e58d8ea5142034e709753))
* **release:** 🔖 0.2.16-rc.8 [skip ci] ([50845f0](https://github.com/inference-gateway/inference-gateway/commit/50845f08c39b940139e8f66cc970ed568e8357db))
* **release:** 🔖 0.2.16-rc.8 [skip ci] ([30d0102](https://github.com/inference-gateway/inference-gateway/commit/30d01026744364e4caa00709ebe516aae070c20d))

## [0.2.16-rc.8](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.7...v0.2.16-rc.8) (2025-03-10)

### 👷 CI

* Add permissions for package management in release workflow ([398df4d](https://github.com/inference-gateway/inference-gateway/commit/398df4dca292f3fc35b81581f50d7a16d91d62fd))

### 🔧 Miscellaneous

* **goreleaser:** Update release mode to keep existing release created by semantic-release ([5424528](https://github.com/inference-gateway/inference-gateway/commit/5424528c2d50d1881a0e58d8ea5142034e709753))
* **release:** 🔖 0.2.16-rc.8 [skip ci] ([30d0102](https://github.com/inference-gateway/inference-gateway/commit/30d01026744364e4caa00709ebe516aae070c20d))

## [0.2.16-rc.8](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.7...v0.2.16-rc.8) (2025-03-10)

### 👷 CI

* Add permissions for package management in release workflow ([398df4d](https://github.com/inference-gateway/inference-gateway/commit/398df4dca292f3fc35b81581f50d7a16d91d62fd))

## [0.2.16-rc.7](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.6...v0.2.16-rc.7) (2025-03-10)

### 👷 CI

* Update Docker image templates to conditionally use 'latest' tag for non-rc versions ([26dc8d7](https://github.com/inference-gateway/inference-gateway/commit/26dc8d7e122b11adaca231aa21c56b003ac896ca))

## [0.2.16-rc.6](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.5...v0.2.16-rc.6) (2025-03-10)

### 👷 CI

* Add GitHub CLI installation to the development container ([5977da6](https://github.com/inference-gateway/inference-gateway/commit/5977da692c5605bf9fa1cb6a6cedac526b781db5))
* Remove version tagging from GoReleaser command in release workflow ([86a99ae](https://github.com/inference-gateway/inference-gateway/commit/86a99ae2b8bed4db68d767e4fb84962ac303705f))
* Update release version format to include 'v' prefix ([4cc3638](https://github.com/inference-gateway/inference-gateway/commit/4cc3638b86192e46916bcf76359382763e52cecb))

## [0.2.16-rc.5](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.4...v0.2.16-rc.5) (2025-03-10)

### 👷 CI

* **fix:** Fetch the current ref which is the branch name then I should have the tags ([adf0318](https://github.com/inference-gateway/inference-gateway/commit/adf031896c74d657ad89dfdb0a2c3f4555f54cf2))

## [0.2.16-rc.4](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.3...v0.2.16-rc.4) (2025-03-10)

### 👷 CI

* Fetch latest tags and update goreleaser command to include version tagging ([2b8bbd0](https://github.com/inference-gateway/inference-gateway/commit/2b8bbd0d1138e9da21fe73c0b281b96f6ebbdc09))

## [0.2.16-rc.3](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.2...v0.2.16-rc.3) (2025-03-10)

### 👷 CI

* Remove git tag and push commands from release workflow ([8ff650e](https://github.com/inference-gateway/inference-gateway/commit/8ff650ec59abe5fa84483a33df5f26389bc6d861))

## [0.2.16-rc.2](https://github.com/inference-gateway/inference-gateway/compare/v0.2.16-rc.1...v0.2.16-rc.2) (2025-03-10)

### 👷 CI

* Add version tagging and push to release workflow ([194e6b9](https://github.com/inference-gateway/inference-gateway/commit/194e6b973dd2a6ba9dd74860656981b2107b2465))

## [0.2.16-rc.1](https://github.com/inference-gateway/inference-gateway/compare/v0.2.15...v0.2.16-rc.1) (2025-03-10)

### 👷 CI

* Change release workflow trigger from manual to push temporarily to test the new workflow ([e94856b](https://github.com/inference-gateway/inference-gateway/commit/e94856bab87eac3396cb8643ab7e846a1ac8fda0))
* Refactor release workflow and add semantic release configuration ([b82b2b1](https://github.com/inference-gateway/inference-gateway/commit/b82b2b105719347156e8c9061ceee0060632042d))

### 📚 Documentation

* Add tool-use support and streaming responses to README ([af990a3](https://github.com/inference-gateway/inference-gateway/commit/af990a371142b726142beb06a08a11552a7abc58))
* Enhance diagram in README ([f98c3ff](https://github.com/inference-gateway/inference-gateway/commit/f98c3ff37b4084daa734c4b812598df60654afb8))
* **examples:** Add TLS configuration setup example for Inference Gateway ([#39](https://github.com/inference-gateway/inference-gateway/issues/39)) ([844df89](https://github.com/inference-gateway/inference-gateway/commit/844df89a5e335419e3d62e1d9240016e36c485d8))
* **fix:** Table was broken, fixed it ([a059a78](https://github.com/inference-gateway/inference-gateway/commit/a059a780f18c06eb2d1c2967e7c6d34fbe8921fd))
* Update OpenTelemetry description for clarity, keep it short and concise ([bd51700](https://github.com/inference-gateway/inference-gateway/commit/bd5170064d20869191c8e35aba5c4f4123ab1994))
