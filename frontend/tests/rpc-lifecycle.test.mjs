import assert from 'node:assert/strict';
import test from 'node:test';
import { mkdtemp, rm } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { build } from 'esbuild';

const cleanups = [];
let sockets = 0;
let requests = 0;
let provided;
const saved = { WebSocket: globalThis.WebSocket, fetch: globalThis.fetch, window: globalThis.window };
globalThis.window = { location: { protocol: 'http:', host: 'localhost', origin: 'http://localhost' } };
class Socket extends EventTarget {
  static OPEN = 1;
  static CLOSED = 3;
  readyState = 0;
  constructor() {
    super();
    sockets++;
    queueMicrotask(() => {
      if (this.readyState === Socket.CLOSED) return;
      this.readyState = Socket.OPEN;
      this.onopen?.(new Event('open'));
      this.dispatchEvent(new Event('open'));
    });
  }
  close() {
    this.readyState = Socket.CLOSED;
    this.onclose?.({ code: 1000 });
  }
  send() {}
}
globalThis.WebSocket = Socket;
globalThis.fetch = async (_url, options) => {
  requests++;
  const request = JSON.parse(options.body);
  return { ok: true, json: async () => ({ jsonrpc: '2.0', id: request.id, result: { version: 'test' } }) };
};
globalThis.__rpcHooks = {
  useState: initializer => [typeof initializer === 'function' ? initializer() : initializer, () => {}],
  useEffect: effect => { const cleanup = effect(); if (cleanup) cleanups.push(cleanup); },
  useCallback: callback => callback,
  useMemo: factory => factory(),
  createContext: () => ({ Provider: 'provider' }),
  createElement: (_type, props) => { provided = props.value; return null; },
};
const directory = await mkdtemp(resolve('tests/.tmp-'));
let RPC2Provider;
try {
  const outfile = resolve(directory, 'module.mjs');
  await build({
    entryPoints: [resolve('src/contexts/RPC2Context.tsx')],
    outfile, bundle: true, platform: 'node', format: 'esm', packages: 'external',
    plugins: [{ name: 'test-stubs', setup(builder) {
      builder.onResolve({ filter: /^react$|i18n\/config$/ }, args => ({ path: args.path, namespace: 'test-stub' }));
      builder.onLoad({ filter: /.*/, namespace: 'test-stub' }, args => ({
        contents: args.path === 'react'
          ? 'const hooks = globalThis.__rpcHooks; export default hooks; export const { useState, useEffect, useCallback, useMemo } = hooks;'
          : 'export default { t: key => key };',
      }));
    } }],
  });
  ({ RPC2Provider } = await import(pathToFileURL(outfile).href));
} finally {
  await rm(directory, { recursive: true, force: true });
}

test('admin provider stays HTTP-only until explicit connect and releases its socket', async () => {
  try {
    RPC2Provider({ children: null });
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(sockets, 0, 'mount must not open an idle socket');
    assert.equal(provided.client.state, 'disconnected');
    assert.deepEqual(await provided.client.call('public:getVersion'), { version: 'test' });
    assert.equal(requests, 1);
    assert.equal(sockets, 0, 'ordinary RPC uses HTTP without connecting');

    await provided.connect();
    assert.equal(sockets, 1, 'explicit connect still works');
    assert.equal(provided.client.state, 'connected');
    for (const cleanup of cleanups.splice(0)) cleanup();
    assert.equal(provided.client.state, 'disconnected');
  } finally {
    for (const cleanup of cleanups.splice(0)) cleanup();
    Object.assign(globalThis, saved);
    delete globalThis.__rpcHooks;
  }
});
