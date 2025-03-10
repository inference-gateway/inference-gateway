# Changelog

All notable changes to this project will be documented in this file.

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
