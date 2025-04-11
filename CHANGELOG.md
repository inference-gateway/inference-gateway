# Changelog

All notable changes to this project will be documented in this file.

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
