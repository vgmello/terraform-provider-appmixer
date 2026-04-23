interface Flow {
  flowId: string;
  name: string;
  flow: { components?: Record<string, unknown>; connections?: unknown[] };
  userId: string;
  sharedWith: Array<{ user: string; permissions: string[] }>;
  stage: "stopped" | "running";
  btime: number;
  mtime: number;
}

interface Store {
  config: Array<{ key: string; value: unknown }>;
  serviceConfig: Array<Record<string, unknown>>;
  acl: Record<string, Array<Record<string, unknown>>>;
  flows: Flow[];
}

const SEED_DATA: Store = {
  config: [
    { key: "API_URL", value: "https://api.example.com" },
    { key: "LOG_LEVEL", value: "info" },
  ],
  serviceConfig: [
    { serviceId: "appmixer:google", clientId: "google-123", clientSecret: "google-secret" },
    { serviceId: "appmixer:slack", clientId: "slack-456", clientSecret: "slack-secret" },
  ],
  acl: {
    components: [
      { role: "admin", resource: "*", action: ["*"], attributes: ["*"] },
    ],
    routes: [
      { role: "user", resource: "/api/*", action: ["read"], attributes: [] },
    ],
  },
  flows: [
    {
      flowId: "flow-seed-1",
      name: "Seed Flow One",
      flow: { components: {}, connections: [] },
      userId: "owner-user",
      sharedWith: [{ user: "alice", permissions: ["read"] }],
      stage: "stopped",
      btime: 1000,
      mtime: 1000,
    },
    {
      flowId: "flow-seed-2",
      name: "Seed Flow Two (running)",
      flow: { components: {}, connections: [] },
      userId: "owner-user",
      sharedWith: [],
      stage: "running",
      btime: 2000,
      mtime: 2000,
    },
  ],
};

const TEST_USER = { username: "admin@test.com", password: "test123" };

let store: Store = structuredClone(SEED_DATA);
let nextFlowId = 1000;

export function resetStore() {
  store = structuredClone(SEED_DATA);
  nextFlowId = 1000;
}

export function getStore(): Store {
  return store;
}

function makeToken(username: string): string {
  const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const payload = btoa(
    JSON.stringify({
      username,
      exp: Math.floor(Date.now() / 1000) + 3600,
    })
  );
  return `${header}.${payload}.mock-signature`;
}

function checkAuth(req: Request): Response | null {
  const auth = req.headers.get("authorization");
  if (!auth || !auth.startsWith("Bearer ")) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }
  return null;
}

function handleRequest(req: Request): Response {
  const url = new URL(req.url);
  const path = url.pathname;
  const method = req.method;

  // POST /user/auth — no auth required
  if (method === "POST" && path === "/user/auth") {
    return handleAuth(req);
  }

  // All other endpoints require auth
  const authError = checkAuth(req);
  if (authError) return authError;

  // Config routes
  if (method === "GET" && path === "/config") {
    return Response.json(store.config);
  }
  if (method === "POST" && path === "/config") {
    return handleConfigCreate(req);
  }
  if (method === "DELETE" && path.startsWith("/config/")) {
    const key = decodeURIComponent(path.slice("/config/".length));
    store.config = store.config.filter((c) => c.key !== key);
    return Response.json({ ok: true });
  }

  // ACL routes
  if (method === "GET" && path === "/acl-types") {
    return Response.json(Object.keys(store.acl));
  }
  if (method === "GET" && path.match(/^\/acl\/[^/]+\/resources$/)) {
    return Response.json(["*", "appmixer:module:*", "appmixer:core:*"]);
  }
  if (method === "GET" && path.match(/^\/acl\/[^/]+\/actions$/)) {
    return Response.json(["*", "read", "!read", "create", "update", "delete"]);
  }
  if (method === "GET" && path.match(/^\/acl\/[^/]+\/resource\/[^/]+\/attributes$/)) {
    return Response.json([]);
  }
  if (method === "GET" && path.match(/^\/acl\/[^/]+$/)) {
    const type = path.slice("/acl/".length);
    const rules = store.acl[type];
    if (!rules) return Response.json({ error: "Not found" }, { status: 404 });
    return Response.json(rules);
  }
  if (method === "POST" && path.match(/^\/acl\/[^/]+$/)) {
    return handleAclUpdate(req, path);
  }

  // Service-config routes
  if (method === "GET" && path === "/service-config") {
    return handleServiceConfigGetAll(url);
  }
  if (method === "POST" && path === "/service-config") {
    return handleServiceConfigCreate(req);
  }
  if (method === "GET" && path.startsWith("/service-config/")) {
    const serviceId = decodeURIComponent(path.slice("/service-config/".length));
    const found = store.serviceConfig.find((s) => s.serviceId === serviceId);
    if (!found) return Response.json({ error: "Not found" }, { status: 404 });
    return Response.json(found);
  }
  if (method === "PUT" && path.startsWith("/service-config/")) {
    return handleServiceConfigUpdate(req, path);
  }
  if (method === "DELETE" && path.startsWith("/service-config/")) {
    const serviceId = decodeURIComponent(path.slice("/service-config/".length));
    store.serviceConfig = store.serviceConfig.filter((s) => s.serviceId !== serviceId);
    return Response.json({});
  }

  // Flows routes
  if (method === "GET" && path === "/flows") {
    return Response.json(store.flows);
  }
  if (method === "POST" && path === "/flows") {
    return handleFlowCreate(req);
  }
  if (method === "GET" && path.match(/^\/flows\/[^/]+$/)) {
    const id = decodeURIComponent(path.slice("/flows/".length));
    const found = store.flows.find((f) => f.flowId === id);
    if (!found) return Response.json({ error: "Not found" }, { status: 404 });
    return Response.json(found);
  }
  if (method === "PUT" && path.match(/^\/flows\/[^/]+$/)) {
    return handleFlowUpdate(req, url, path);
  }
  if (method === "DELETE" && path.match(/^\/flows\/[^/]+$/)) {
    const id = decodeURIComponent(path.slice("/flows/".length));
    const before = store.flows.length;
    store.flows = store.flows.filter((f) => f.flowId !== id);
    if (store.flows.length === before) {
      return Response.json({ error: "Not found" }, { status: 404 });
    }
    return Response.json({});
  }

  return Response.json({ error: "Not found" }, { status: 404 });
}

async function handleAuth(req: Request): Promise<Response> {
  const body = await req.json();
  if (body.username === TEST_USER.username && body.password === TEST_USER.password) {
    return Response.json({ token: makeToken(body.username), username: body.username });
  }
  return Response.json({ error: "Invalid credentials" }, { status: 401 });
}

async function handleConfigCreate(req: Request): Promise<Response> {
  const body = await req.json();
  store.config.push(body);
  return Response.json(body);
}

async function handleAclUpdate(req: Request, path: string): Promise<Response> {
  const type = path.slice("/acl/".length);
  const body = await req.json();
  store.acl[type] = body;
  return new Response(null, { status: 200 });
}

function handleServiceConfigGetAll(url: URL): Response {
  let results = [...store.serviceConfig];
  const pattern = url.searchParams.get("pattern");
  if (pattern) {
    results = results.filter((s) =>
      (s.serviceId as string).includes(pattern)
    );
  }
  const offset = parseInt(url.searchParams.get("offset") ?? "0", 10);
  const limit = parseInt(url.searchParams.get("limit") ?? "100", 10);
  results = results.slice(offset, offset + limit);
  return Response.json(results);
}

async function handleServiceConfigCreate(req: Request): Promise<Response> {
  const body = await req.json();
  store.serviceConfig.push(body);
  return Response.json(body);
}

async function handleServiceConfigUpdate(req: Request, path: string): Promise<Response> {
  const serviceId = decodeURIComponent(path.slice("/service-config/".length));
  const body = await req.json();
  const idx = store.serviceConfig.findIndex((s) => s.serviceId === serviceId);
  if (idx === -1) return Response.json({ error: "Not found" }, { status: 404 });
  store.serviceConfig[idx] = body;
  return Response.json(body);
}

async function handleFlowCreate(req: Request): Promise<Response> {
  const body = (await req.json()) as Partial<Flow>;
  const now = Date.now();
  const created: Flow = {
    flowId: `flow-${nextFlowId++}`,
    name: body.name ?? "Untitled",
    flow: body.flow ?? { components: {}, connections: [] },
    userId: "owner-user",
    sharedWith: [],
    stage: "stopped",
    btime: now,
    mtime: now,
  };
  store.flows.push(created);
  return Response.json(created);
}

async function handleFlowUpdate(req: Request, url: URL, path: string): Promise<Response> {
  const id = decodeURIComponent(path.slice("/flows/".length));
  const idx = store.flows.findIndex((f) => f.flowId === id);
  if (idx === -1) return Response.json({ error: "Not found" }, { status: 404 });

  const current = store.flows[idx]!;
  const force = url.searchParams.get("forceUpdate") === "true";
  if (current.stage === "running" && !force) {
    return Response.json(
      { error: "Flow is running. Pass forceUpdate=true." },
      { status: 409 }
    );
  }

  const body = (await req.json()) as Partial<Flow>;
  store.flows[idx] = {
    ...current,
    name: body.name ?? current.name,
    flow: body.flow ?? current.flow,
    mtime: Date.now(),
  };
  return Response.json(store.flows[idx]);
}

export function startServer(port: number = 0) {
  const server = Bun.serve({
    port,
    fetch: handleRequest,
  });
  return {
    port: server.port,
    stop: () => server.stop(),
  };
}

// Standalone mode
if (import.meta.main) {
  const envPort = Number(process.env.PORT);
  const { port } = startServer(Number.isFinite(envPort) ? envPort : 3000);
  console.log(`Mock Appmixer API running on http://localhost:${port}`);
}
