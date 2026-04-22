#!/usr/bin/env node

import { randomBytes } from 'node:crypto';
import { EventEmitter, once } from 'node:events';
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';
import { spawn } from 'node:child_process';
import { setTimeout as delay } from 'node:timers/promises';

function usage() {
  process.stderr.write(
    'Usage: node scripts/smoke_prod_deploy_helper.mjs <base-url> <output-dir> [resolve-ip]\n',
  );
}

function die(message) {
  process.stderr.write(`smoke-helper: ERROR: ${message}\n`);
  process.exit(1);
}

function log(message) {
  process.stderr.write(`smoke-helper: ${message}\n`);
}

class CDPClient extends EventEmitter {
  constructor(wsUrl) {
    super();
    this.wsUrl = wsUrl;
    this.ws = null;
    this.nextId = 1;
    this.pending = new Map();
  }

  async connect() {
    this.ws = new WebSocket(this.wsUrl);
    this.ws.addEventListener('message', (event) => {
      const message = JSON.parse(event.data.toString());
      if (typeof message.id === 'number') {
        const pending = this.pending.get(message.id);
        if (!pending) {
          return;
        }
        this.pending.delete(message.id);
        if (message.error) {
          pending.reject(new Error(message.error.message ?? JSON.stringify(message.error)));
          return;
        }
        pending.resolve(message.result ?? {});
        return;
      }

      if (message.method) {
        this.emit(message.method, message.params ?? {});
      }
    });
    this.ws.addEventListener('close', () => {
      for (const pending of this.pending.values()) {
        pending.reject(new Error('CDP websocket closed'));
      }
      this.pending.clear();
    });
    this.ws.addEventListener('error', (event) => {
      this.emit('socket-error', event.error ?? new Error('CDP websocket error'));
    });
    await once(this.ws, 'open');
  }

  async close() {
    if (!this.ws) {
      return;
    }
    this.ws.close();
    await once(this.ws, 'close').catch(() => undefined);
  }

  async send(method, params = {}) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error(`CDP websocket is not open for method ${method}`);
    }

    const id = this.nextId++;
    const payload = JSON.stringify({ id, method, params });
    const result = new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
    });
    this.ws.send(payload);
    return result;
  }
}

async function waitFor(check, timeoutMs, description) {
  const deadline = Date.now() + timeoutMs;
  // eslint-disable-next-line no-constant-condition
  while (true) {
    const result = await check();
    if (result) {
      return result;
    }
    if (Date.now() >= deadline) {
      throw new Error(`timed out waiting for ${description}`);
    }
    await delay(200);
  }
}

async function fetchJSON(url) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`HTTP ${response.status} from ${url}`);
  }
  return response.json();
}

function escapeForTemplate(value) {
  return JSON.stringify(value);
}

async function evaluate(cdp, expression, awaitPromise = true) {
  const result = await cdp.send('Runtime.evaluate', {
    expression,
    awaitPromise,
    returnByValue: true,
  });
  if (result.exceptionDetails) {
    const message = result.exceptionDetails.text || 'Runtime.evaluate failed';
    throw new Error(message);
  }
  return result.result?.value;
}

async function navigate(cdp, url) {
  const loadEvent = once(cdp, 'Page.loadEventFired');
  await cdp.send('Page.navigate', { url });
  await Promise.race([
    loadEvent,
    delay(15000).then(() => {
      throw new Error(`timed out waiting for load event: ${url}`);
    }),
  ]);
  await delay(500);
}

async function waitForSelector(cdp, selector, timeoutMs = 15000) {
  return waitFor(
    async () =>
      evaluate(
        cdp,
        `Boolean(document.querySelector(${escapeForTemplate(selector)}))`,
      ),
    timeoutMs,
    `selector ${selector}`,
  );
}

async function waitForLocalStorageValue(cdp, key, timeoutMs = 15000) {
  return waitFor(
    async () => evaluate(cdp, `localStorage.getItem(${escapeForTemplate(key)})`),
    timeoutMs,
    `localStorage key ${key}`,
  );
}

async function waitForAuthFlowCompletion(cdp, state, requestPath, expectedStatus, timeoutMs = 30000) {
  return waitFor(async () => {
    const request = summarizeRequest(state, requestPath);
    const currentPath = await evaluate(cdp, 'window.location.pathname');
    const token = await evaluate(cdp, `localStorage.getItem('gm_token')`);

    if (currentPath === '/' && (request?.status === expectedStatus || typeof token === 'string')) {
      return {
        request,
        currentPath,
        token,
      };
    }

    return null;
  }, timeoutMs, `${requestPath} completion`);
}

async function fillAndSubmitForm(cdp, valuesBySelector) {
  await evaluate(
    cdp,
    `(() => {
      const values = ${JSON.stringify(valuesBySelector)};
      const inputSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
      for (const [selector, value] of Object.entries(values)) {
        const element = document.querySelector(selector);
        if (!element) {
          throw new Error('missing form field: ' + selector);
        }
        inputSetter.call(element, value);
        element.dispatchEvent(new Event('input', { bubbles: true }));
        element.dispatchEvent(new Event('change', { bubbles: true }));
      }
      const form = document.querySelector('form');
      if (!form) {
        throw new Error('missing form');
      }
      form.requestSubmit();
      return true;
    })()`,
  );
}

function recordRequest(state, params) {
  const entry = state.requestsById.get(params.requestId) ?? {
    requestId: params.requestId,
  };
  entry.url = params.request.url;
  entry.method = params.request.method;
  entry.type = params.type ?? entry.type ?? null;
  entry.documentURL = params.documentURL ?? entry.documentURL ?? null;
  entry.hasPostData = Boolean(params.request.hasPostData);
  entry.initiator = params.initiator?.type ?? entry.initiator ?? null;
  state.requestsById.set(params.requestId, entry);
}

function recordResponse(state, params) {
  const entry = state.requestsById.get(params.requestId) ?? {
    requestId: params.requestId,
  };
  entry.status = params.response.status;
  entry.statusText = params.response.statusText;
  entry.mimeType = params.response.mimeType;
  entry.remoteIPAddress = params.response.remoteIPAddress ?? null;
  entry.responseHeaders = params.response.headers ?? {};
  state.requestsById.set(params.requestId, entry);
}

function summarizeRequest(state, pathname) {
  for (const request of state.requestsById.values()) {
    if (!request.url) {
      continue;
    }
    const url = new URL(request.url);
    if (url.pathname === pathname) {
      return request;
    }
  }
  return null;
}

function extractWebSocketEventPayload(params) {
  if (!params?.response?.payloadData) {
    return null;
  }

  const text = params.response.payloadData;
  const parsed = JSON.parse(text);
  if (!parsed || typeof parsed !== 'object' || typeof parsed.type !== 'string') {
    throw new Error('browser websocket frame was not a valid event envelope');
  }

  return {
    text,
    parsed,
  };
}

function requestTextMatchesWebSocketPath(requestText, wsPath) {
  if (typeof requestText !== 'string') {
    return false;
  }

  return requestText.includes(wsPath);
}

async function waitForBrowserWebSocket(state, campaignId, timeoutMs = 15000) {
  const wsPath = `/api/v1/campaigns/${campaignId}/ws`;

  return waitFor(async () => {
    const handshakeResponse = state.webSocketEvents.find(({ event, params }) => {
      if (event !== 'handshakeResponse' || params?.response?.status !== 101) {
        return false;
      }

      const requestText = params?.response?.requestHeadersText;
      return requestTextMatchesWebSocketPath(requestText, wsPath);
    });

    if (!handshakeResponse) {
      return null;
    }

    const requestId = handshakeResponse.params.requestId;
    const handshakeRequest = state.webSocketEvents.find(
      ({ event, params }) => event === 'handshakeRequest' && params?.requestId === requestId,
    );
    const frameEvent = state.webSocketEvents.find(
      ({ event, params }) => event === 'frameReceived' && params?.requestId === requestId,
    );

    if (!frameEvent) {
      return null;
    }

    const payload = extractWebSocketEventPayload(frameEvent.params);
    if (!payload) {
      return null;
    }

    return {
      endpoint: wsPath,
      requestId,
      handshakeStatus: handshakeResponse.params.response.status,
      handshakeHeaders: handshakeResponse.params.response.headers,
      requestHeaders: handshakeRequest?.params?.request?.headers ?? {},
      firstMessageText: payload.text,
      firstMessage: payload.parsed,
    };
  }, timeoutMs, 'browser websocket handshake and first frame');
}

async function waitForTrackedBrowserWebSocketOpen(cdp, campaignId, timeoutMs = 15000) {
  const wsPath = `/api/v1/campaigns/${campaignId}/ws`;

  return waitFor(async () => {
    const trackedSocket = await evaluate(
      cdp,
      `(() => {
        const sockets = Array.isArray(window.__eddaSmokeSockets) ? window.__eddaSmokeSockets : [];
        const match = sockets.find((socket) => typeof socket.url === 'string' && socket.url.includes(${escapeForTemplate(wsPath)}));
        if (!match) {
          return null;
        }

        return {
          url: match.url,
          readyState: match.readyState,
          openAt: match.openAt,
          closeAt: match.closeAt,
          sentCount: Array.isArray(match.sentMessages) ? match.sentMessages.length : 0,
        };
      })()`,
    );

    if (!trackedSocket || trackedSocket.readyState !== WebSocket.OPEN || typeof trackedSocket.openAt !== 'number') {
      return null;
    }

    return trackedSocket;
  }, timeoutMs, 'tracked browser websocket open state');
}

async function launchChrome(baseUrl, resolveIp) {
  const userDataDir = await mkdtemp(path.join(os.tmpdir(), 'edda-smoke-chrome-'));
  const port = 9222 + Math.floor(Math.random() * 4000);
  const args = [
    `--remote-debugging-port=${port}`,
    '--headless=new',
    '--disable-gpu',
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-background-networking',
    '--disable-background-timer-throttling',
    '--disable-renderer-backgrounding',
    '--disable-sync',
    `--user-data-dir=${userDataDir}`,
    'about:blank',
  ];
  if (resolveIp) {
    const hostname = new URL(baseUrl).hostname;
    args.unshift(`--host-resolver-rules=MAP ${hostname} ${resolveIp}`);
  }

  const chrome = spawn('google-chrome', args, {
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  let stderr = '';
  chrome.stderr.on('data', (chunk) => {
    stderr += chunk.toString('utf8');
  });
  chrome.stdout.on('data', () => {});

  await waitFor(async () => {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/version`);
      return response.ok;
    } catch {
      if (chrome.exitCode !== null) {
        throw new Error(`chrome exited early: ${stderr.trim()}`);
      }
      return false;
    }
  }, 15000, 'chrome devtools endpoint');

  return { chrome, userDataDir, port };
}

async function writeDebugArtifacts(outputDir, state, extra = {}) {
  await writeFile(
    path.join(outputDir, 'browser-network.json'),
    JSON.stringify([...state.requestsById.values()], null, 2),
    'utf8',
  );
  await writeFile(
    path.join(outputDir, 'browser-websocket-events.json'),
    JSON.stringify(state.webSocketEvents, null, 2),
    'utf8',
  );
  await writeFile(
    path.join(outputDir, 'browser-debug.json'),
    JSON.stringify(extra, null, 2),
    'utf8',
  );
}

async function main() {
  if (process.argv.length < 4 || process.argv.length > 5) {
    usage();
    process.exit(1);
  }

  const baseUrl = process.argv[2];
  const outputDir = process.argv[3];
  const resolveIp = process.argv[4] || '';
  const parsedBase = new URL(baseUrl);

  if (!['https:', 'http:'].includes(parsedBase.protocol)) {
    die(`unsupported base URL protocol: ${parsedBase.protocol}`);
  }

  await mkdir(outputDir, { recursive: true });

  const { chrome, userDataDir, port } = await launchChrome(baseUrl, resolveIp);
  const cleanup = async () => {
    chrome.kill('SIGTERM');
    await once(chrome, 'exit').catch(() => undefined);
    await rm(userDataDir, { recursive: true, force: true });
  };

  try {
    const targets = await fetchJSON(`http://127.0.0.1:${port}/json`);
    const pageTarget = targets.find((target) => target.type === 'page' && target.webSocketDebuggerUrl);
    if (!pageTarget) {
      throw new Error('could not find Chrome page target');
    }

    const cdp = new CDPClient(pageTarget.webSocketDebuggerUrl);
    const state = {
      requestsById: new Map(),
      webSocketEvents: [],
    };

    try {
      await cdp.connect();

      cdp.on('Network.requestWillBeSent', (params) => recordRequest(state, params));
      cdp.on('Network.responseReceived', (params) => recordResponse(state, params));
      cdp.on('Network.loadingFailed', (params) => {
        const entry = state.requestsById.get(params.requestId) ?? { requestId: params.requestId };
        entry.loadingFailed = params.errorText;
        state.requestsById.set(params.requestId, entry);
      });
      cdp.on('Network.webSocketWillSendHandshakeRequest', (params) => {
        state.webSocketEvents.push({ event: 'handshakeRequest', params });
      });
      cdp.on('Network.webSocketHandshakeResponseReceived', (params) => {
        state.webSocketEvents.push({ event: 'handshakeResponse', params });
      });
      cdp.on('Network.webSocketFrameReceived', (params) => {
        state.webSocketEvents.push({ event: 'frameReceived', params });
      });

      await cdp.send('Page.enable');
      await cdp.send('Runtime.enable');
      await cdp.send('Network.enable');
      await cdp.send('Page.addScriptToEvaluateOnNewDocument', {
        source: `(() => {
          const NativeWebSocket = window.WebSocket;
          const trackedSockets = [];
          Object.defineProperty(window, '__eddaSmokeSockets', {
            configurable: false,
            enumerable: false,
            writable: false,
            value: trackedSockets,
          });

          window.WebSocket = class SmokeTrackedWebSocket extends NativeWebSocket {
            constructor(url, protocols) {
              super(url, protocols);

              const entry = {
                url: String(url),
                readyState: this.readyState,
                openAt: null,
                closeAt: null,
                errorAt: null,
                sentMessages: [],
              };
              trackedSockets.push(entry);

              const updateReadyState = () => {
                entry.readyState = this.readyState;
              };

              this.addEventListener('open', () => {
                updateReadyState();
                entry.openAt = Date.now();
              });
              this.addEventListener('close', () => {
                updateReadyState();
                entry.closeAt = Date.now();
              });
              this.addEventListener('error', () => {
                updateReadyState();
                entry.errorAt = Date.now();
              });

              const nativeSend = this.send.bind(this);
              this.send = (data) => {
                entry.sentMessages.push(typeof data === 'string' ? data : String(data));
                updateReadyState();
                return nativeSend(data);
              };
            }
          };
        })();`,
      });

      const email = `prod-smoke+${Date.now()}-${randomBytes(3).toString('hex')}@example.com`;
      const password = 'S3curePass!234';
      const name = 'Production Smoke';

      log(`registering smoke user ${email}`);
      await navigate(cdp, new URL('/register', baseUrl).toString());
      await waitForSelector(cdp, 'input[autocomplete="name"]');
      await fillAndSubmitForm(cdp, {
        'input[autocomplete="name"]': name,
        'input[autocomplete="email"]': email,
        'input[autocomplete="new-password"]': password,
      });
      await waitForAuthFlowCompletion(cdp, state, '/api/v1/auth/register', 201, 30000);

      await evaluate(cdp, 'localStorage.clear(); true;');
      await navigate(cdp, new URL('/login', baseUrl).toString());
      await waitForSelector(cdp, 'input[autocomplete="email"]');
      await fillAndSubmitForm(cdp, {
        'input[autocomplete="email"]': email,
        'input[autocomplete="current-password"]': password,
      });
      await waitForAuthFlowCompletion(cdp, state, '/api/v1/auth/login', 200, 30000);
      const authToken = await waitForLocalStorageValue(cdp, 'gm_token', 30000);
      if (typeof authToken !== 'string' || authToken.trim() === '') {
        throw new Error('expected authenticated browser session to populate localStorage gm_token');
      }

      const authMe = await evaluate(
        cdp,
        `(() => fetch('/api/v1/auth/me', {
          credentials: 'include',
          headers: {
            Accept: 'application/json',
            Authorization: 'Bearer ' + localStorage.getItem('gm_token'),
          },
        }).then(async (response) => ({
          status: response.status,
          body: await response.text(),
        })))()`,
      );
      if (authMe.status !== 200) {
        throw new Error(`expected /api/v1/auth/me to return 200, got ${authMe.status}`);
      }
      const authMeBody = JSON.parse(authMe.body);

      const campaignLookup = await evaluate(
        cdp,
        `(() => {
          const token = localStorage.getItem('gm_token');
          const headers = {
            Accept: 'application/json',
            Authorization: 'Bearer ' + token,
          };
          return fetch('/api/v1/campaigns', { credentials: 'include', headers }).then(async (response) => {
            const text = await response.text();
            if (!response.ok) {
              return { listStatus: response.status, listBody: text };
            }
            const parsed = JSON.parse(text || '{}');
            const first = Array.isArray(parsed.campaigns) ? parsed.campaigns[0] : undefined;
            if (first && first.id) {
              return {
                listStatus: response.status,
                campaignsCount: parsed.campaigns.length,
                campaignId: first.id,
                created: false,
              };
            }

            return fetch('/api/v1/campaigns', {
              method: 'POST',
              credentials: 'include',
              headers: {
                ...headers,
                'Content-Type': 'application/json',
              },
              body: JSON.stringify({
                name: 'Production Smoke Campaign',
                description: '',
                genre: '',
                tone: '',
                themes: [],
              }),
            }).then(async (createResponse) => ({
              listStatus: response.status,
              createStatus: createResponse.status,
              created: true,
              campaign: JSON.parse(await createResponse.text()),
            }));
          });
        })()`,
      );
      if (campaignLookup.listStatus !== 200) {
        throw new Error(`campaign lookup failed with status ${campaignLookup.listStatus}`);
      }

      const campaignId = campaignLookup.created ? campaignLookup.campaign.id : campaignLookup.campaignId;
      if (!campaignId) {
        throw new Error('failed to obtain a concrete campaign id for websocket smoke');
      }

      const apiRequests = [...state.requestsById.values()]
        .filter((request) => {
          if (!request.url) {
            return false;
          }
          const url = new URL(request.url);
          return url.pathname.startsWith('/api/');
        })
        .map((request) => ({
          ...request,
          isSameOrigin: new URL(request.url).origin === parsedBase.origin,
        }));
      if (apiRequests.length === 0) {
        throw new Error('browser session did not emit any /api/* requests');
      }
      if (apiRequests.some((request) => request.isSameOrigin !== true)) {
        throw new Error('browser session emitted a non-same-origin API request');
      }

      await navigate(cdp, new URL(`/play/${campaignId}`, baseUrl).toString());
      await waitForSelector(cdp, 'textarea');
      await waitForTrackedBrowserWebSocketOpen(cdp, campaignId, 30000);
      await delay(250);

      await evaluate(
        cdp,
        `(() => {
          const textarea = document.querySelector('textarea');
          if (!textarea) {
            throw new Error('missing action textarea');
          }
          const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
          setter.call(textarea, 'look around');
          textarea.dispatchEvent(new Event('input', { bubbles: true }));
          const submit = document.querySelector('button[type="submit"]');
          if (!(submit instanceof HTMLButtonElement)) {
            throw new Error('missing send action button');
          }
          submit.click();
          return true;
        })()`,
      );

      const websocket = await waitForBrowserWebSocket(state, campaignId, 30000);

      const summary = {
        baseUrl,
        resolveIp,
        email,
        name,
        register: summarizeRequest(state, '/api/v1/auth/register'),
        login: summarizeRequest(state, '/api/v1/auth/login'),
        authMe: {
          status: authMe.status,
          body: authMeBody,
        },
        campaignLookup,
        campaignId,
        sameOriginApiRequests: apiRequests,
        websocket,
      };

      await writeDebugArtifacts(outputDir, state, {
        finalPath: await evaluate(cdp, 'window.location.pathname'),
        summary,
      });
      await writeFile(
        path.join(outputDir, 'browser-summary.json'),
        JSON.stringify(summary, null, 2),
        'utf8',
      );
    } catch (error) {
      await writeDebugArtifacts(outputDir, state, {
        finalPath: await evaluate(cdp, 'window.location.pathname').catch(() => null),
        error: error instanceof Error ? error.message : String(error),
      }).catch(() => undefined);
      throw error;
    } finally {
      await cdp.close().catch(() => undefined);
      await cleanup();
    }
  } catch (error) {
    await cleanup().catch(() => undefined);
    throw error;
  }
}

main().catch((error) => {
  die(error instanceof Error ? error.message : String(error));
});
