// Downloads the inference-gateway binary that matches the current package
// version (and host platform) from GitHub releases, caches it under the
// user's cache directory, and returns the resolved binary path.
//
// The release archives are produced by goreleaser and follow the naming
// convention: inference-gateway_<OS>_<ARCH>.tar.gz, where:
//   OS:   Linux | Darwin
//   ARCH: x86_64 | arm64 | armv7

'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const https = require('https');
const { execFileSync } = require('child_process');

const pkg = require('../package.json');

const REPO = 'inference-gateway/inference-gateway';

function detectPlatform() {
  let osPart;
  switch (process.platform) {
    case 'linux':
      osPart = 'Linux';
      break;
    case 'darwin':
      osPart = 'Darwin';
      break;
    default:
      throw new Error(
        `Unsupported operating system: ${process.platform}. ` +
          'Only linux and darwin are supported via npx. ' +
          'See https://github.com/inference-gateway/inference-gateway#installation for alternatives.'
      );
  }

  let archPart;
  switch (process.arch) {
    case 'x64':
      archPart = 'x86_64';
      break;
    case 'arm64':
      archPart = 'arm64';
      break;
    case 'arm':
      archPart = 'armv7';
      break;
    default:
      throw new Error(`Unsupported CPU architecture: ${process.arch}.`);
  }

  if (osPart === 'Darwin' && archPart === 'armv7') {
    throw new Error('Darwin/armv7 is not a supported release target.');
  }

  return { os: osPart, arch: archPart };
}

function resolveVersion() {
  const override = process.env.INFERENCE_GATEWAY_VERSION;
  if (override) {
    return override.startsWith('v') ? override : `v${override}`;
  }
  return pkg.version.startsWith('v') ? pkg.version : `v${pkg.version}`;
}

function cacheRoot() {
  if (process.env.INFERENCE_GATEWAY_CACHE_DIR) {
    return process.env.INFERENCE_GATEWAY_CACHE_DIR;
  }
  // XDG-ish default; falls back to ~/.cache on macOS too for simplicity.
  const xdg = process.env.XDG_CACHE_HOME;
  const base = xdg && xdg.length > 0 ? xdg : path.join(os.homedir(), '.cache');
  return path.join(base, 'inference-gateway');
}

function fetchFollow(url) {
  return new Promise((resolve, reject) => {
    const req = https.get(
      url,
      {
        headers: {
          'User-Agent': `npx-inference-gateway/${pkg.version}`,
          Accept: 'application/octet-stream',
        },
      },
      (res) => {
        if (
          res.statusCode &&
          res.statusCode >= 300 &&
          res.statusCode < 400 &&
          res.headers.location
        ) {
          // Drain and follow.
          res.resume();
          fetchFollow(res.headers.location).then(resolve, reject);
          return;
        }
        if (res.statusCode !== 200) {
          reject(
            new Error(
              `Download failed: HTTP ${res.statusCode} for ${url}`
            )
          );
          res.resume();
          return;
        }
        resolve(res);
      }
    );
    req.on('error', reject);
  });
}

async function downloadToFile(url, dest) {
  const res = await fetchFollow(url);
  await new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    res.pipe(file);
    file.on('finish', () => file.close(resolve));
    file.on('error', (err) => {
      fs.unlink(dest, () => reject(err));
    });
    res.on('error', reject);
  });
}

async function resolveBinary() {
  const version = resolveVersion();
  const { os: osPart, arch: archPart } = detectPlatform();

  const root = cacheRoot();
  const versionDir = path.join(root, version);
  const binaryPath = path.join(versionDir, 'inference-gateway');

  if (fs.existsSync(binaryPath)) {
    return binaryPath;
  }

  fs.mkdirSync(versionDir, { recursive: true });

  const archiveName = `inference-gateway_${osPart}_${archPart}.tar.gz`;
  const url = `https://github.com/${REPO}/releases/download/${version}/${archiveName}`;
  const archivePath = path.join(versionDir, archiveName);

  process.stderr.write(
    `[inference-gateway] Downloading ${version} for ${osPart}/${archPart}...\n`
  );

  try {
    await downloadToFile(url, archivePath);
  } catch (err) {
    // Clean up partial download.
    try {
      fs.unlinkSync(archivePath);
    } catch (_) {
      /* ignore */
    }
    throw new Error(
      `Failed to download ${url}: ${err.message}. ` +
        'Check your network connection or set INFERENCE_GATEWAY_VERSION to a known-good release tag.'
    );
  }

  process.stderr.write('[inference-gateway] Extracting...\n');
  try {
    execFileSync('tar', ['-xzf', archivePath, '-C', versionDir], {
      stdio: ['ignore', 'ignore', 'inherit'],
    });
  } catch (err) {
    throw new Error(
      `Failed to extract ${archivePath} (is 'tar' on your PATH?): ${err.message}`
    );
  } finally {
    try {
      fs.unlinkSync(archivePath);
    } catch (_) {
      /* ignore */
    }
  }

  if (!fs.existsSync(binaryPath)) {
    throw new Error(
      `Extracted archive did not contain an 'inference-gateway' binary at ${binaryPath}`
    );
  }
  fs.chmodSync(binaryPath, 0o755);
  return binaryPath;
}

module.exports = { resolveBinary, detectPlatform, resolveVersion };
