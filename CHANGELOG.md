# Changelog

All notable changes to this project will be documented in this file.

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
