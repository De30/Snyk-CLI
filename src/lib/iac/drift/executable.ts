import * as debugLib from 'debug';
import * as os from 'os';
import * as fs from 'fs';
import { spinner } from '../../spinner';
import { makeRequest } from '../../request';
import * as crypto from 'crypto';
import * as path from 'path';
import envPaths from 'env-paths';
import config from '../../../lib/config';
import { createIfNotExists } from './util';

const debug = debugLib('drift');

export const cachePath = config.CACHE_PATH ?? envPaths('snyk').cache;

export const driftctlVersion = 'v0.21.0';
const driftctlChecksums = {
  'driftctl_windows_386.exe':
    'f7affbe0ba270b0339d0398befc686bd747a1694a4db0890ce73a2b1921521d4',
  driftctl_darwin_amd64:
    '85cd7a0b5670aa0ee52181357ec891f5a84df29cf505344e403b8b9de5953d61',
  driftctl_linux_386:
    '4221b4f2db65163ccfd300f90fe7cffa7bac1f806f56971ebbca5e36269aa3a4',
  driftctl_linux_amd64:
    'eb64c0d7a7094f0d741abae24c59a46db3eb76f619f177fd745efea7d468a66e',
  driftctl_linux_arm64:
    'c0c4dbfb2f5217124d3f7e1ef33b8b547fc84adf65612aca438e48e63da2f63e',
  'driftctl_windows_arm64.exe':
    '9e87c2a7fecca5a2846c87d4c570b5357e892e4234d7eafa0dac5ea31142e992',
  driftctl_darwin_arm64:
    '39813b4f05c034b6833508062f72bc17f1edbe2bc4db244893e75198eb013a34',
  'driftctl_windows_arm.exe':
    '5d66cb4db95bfa33d4946d324c4674f10fde8370dfb5003d99242a560d8e7e1b',
  driftctl_linux_arm:
    '13705de80f0de3d1a931e81947cc7a443dcec59968bafcb8ea888a4f643e5605',
  'driftctl_windows_amd64.exe':
    '154afbf87a3c0d36a345ccadad8ca7f85855a1c1f8f622ce1ea46931dadafce7',
};

const dctlBaseUrl = 'https://github.com/snyk/driftctl/releases/download/';
const driftctlPath = path.join(cachePath, 'driftctl_' + driftctlVersion);

export async function findOrDownload(): Promise<string> {
  let dctl = await findDriftCtl();
  if (dctl === '') {
    try {
      createIfNotExists(cachePath);
      dctl = driftctlPath;
      await download(driftctlUrl(), dctl);
    } catch (err) {
      return Promise.reject(err);
    }
  }
  return dctl;
}

async function findDriftCtl(): Promise<string> {
  // lookup in custom path contained in env var DRIFTCTL_PATH
  let dctlPath = config.DRIFTCTL_PATH;
  if (dctlPath != null) {
    const exists = await isExe(dctlPath);
    if (exists) {
      debug('Found driftctl in $DRIFTCTL_PATH: %s', dctlPath);
      return dctlPath;
    }
  }

  // lookup in app cache
  dctlPath = driftctlPath;
  const exists = await isExe(dctlPath);
  if (exists) {
    debug('Found driftctl in cache: %s', dctlPath);
    return dctlPath;
  }
  debug('driftctl not found');

  return '';
}

async function download(url, destination: string): Promise<boolean> {
  debug('downloading driftctl into %s', destination);

  const payload = {
    method: 'GET',
    url: url,
    output: destination,
    follow: 3,
  };

  await spinner('Downloading...');
  return new Promise<boolean>((resolve, reject) => {
    makeRequest(payload, function(err, res, body) {
      try {
        if (err) {
          reject(
            new Error('Could not download driftctl from ' + url + ': ' + err),
          );
          return;
        }
        if (res.statusCode !== 200) {
          reject(
            new Error(
              'Could not download driftctl from ' + url + ': ' + res.statusCode,
            ),
          );
          return;
        }

        validateChecksum(body);

        fs.writeFileSync(destination, body);
        debug('File saved: ' + destination);

        fs.chmodSync(destination, 0o744);
        resolve(true);
      } finally {
        spinner.clearAll();
      }
    });
  });
}

function validateChecksum(body: string) {
  // only validate if we downloaded the official driftctl binary
  if (config.DRIFTCTL_URL || config.DRIFTCTL_PATH) {
    return;
  }

  const computedHash = crypto
    .createHash('sha256')
    .update(body)
    .digest('hex');
  const givenHash = driftctlChecksums[driftctlFileName()];

  if (computedHash != givenHash) {
    throw new Error('Downloaded file has inconsistent checksum...');
  }
}
function driftctlFileName(): string {
  let platform = 'linux';
  switch (os.platform()) {
    case 'darwin':
      platform = 'darwin';
      break;
    case 'win32':
      platform = 'windows';
      break;
  }

  let arch = 'amd64';
  switch (os.arch()) {
    case 'ia32':
    case 'x32':
      arch = '386';
      break;
    case 'arm':
      arch = 'arm';
      break;
    case 'arm64':
      arch = 'arm64';
      break;
  }

  let ext = '';
  switch (os.platform()) {
    case 'win32':
      ext = '.exe';
      break;
  }

  return `driftctl_${platform}_${arch}${ext}`;
}

function driftctlUrl(): string {
  if (config.DRIFTCTL_URL) {
    return config.DRIFTCTL_URL;
  }

  return `${dctlBaseUrl}/${driftctlVersion}/${driftctlFileName()}`;
}

function isExe(dctlPath: string): Promise<boolean> {
  return new Promise<boolean>((resolve) => {
    fs.access(dctlPath, fs.constants.X_OK, (err) => {
      if (err) {
        resolve(false);
        return;
      }
      resolve(true);
    });
  });
}
