import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import test from 'node:test';
import { loadConfigFromFile } from 'vite';

test('development theme endpoint serves the canonical manifest', async () => {
  const loaded = await loadConfigFromFile({ command: 'serve', mode: 'development' }, resolve('vite.config.ts'));
  assert.ok(loaded);
  const plugin = loaded.config.plugins.find(item => item?.name === 'local-komari-theme');
  assert.ok(plugin);
  let middleware;
  plugin.configureServer({ middlewares: { use(handler) { middleware = handler; } } });
  let nextCalls = 0;
  middleware({ url: '/unrelated' }, {}, () => { nextCalls++; });
  assert.equal(nextCalls, 1);
  const headers = {};
  const response = { setHeader(key, value) { headers[key] = value; } };
  const body = await new Promise((done, reject) => {
    response.end = done;
    middleware({ url: '/themes/default/komari-theme.json?fresh=1' }, response, () => reject(new Error('manifest request bypassed')));
  });
  assert.equal(response.statusCode, 200);
  assert.equal(headers['Cache-Control'], 'no-store');
  const canonical = JSON.parse(await readFile(resolve('../theme/komari-theme.json'), 'utf8'));
  assert.deepEqual(JSON.parse(String(body)), canonical);
});

test('embedded theme manifest matches packaging input', async () => {
  const canonical = JSON.parse(await readFile(resolve('../theme/komari-theme.json'), 'utf8'));
  const embedded = JSON.parse(await readFile(resolve('../web/public/defaultTheme/komari-theme.json'), 'utf8'));
  assert.deepEqual(embedded, canonical);
});
